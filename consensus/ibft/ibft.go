package ibft

import (
	"errors"
	"fmt"
	"time"

	"event-pool/helper/progress"
	"go.uber.org/zap"

	"event-pool/blockchain"
	"event-pool/consensus"
	"event-pool/consensus/ibft/fork"
	"event-pool/consensus/ibft/proto"
	"event-pool/consensus/ibft/signer"
	"event-pool/network"
	"event-pool/secrets"
	"event-pool/syncer"
	"event-pool/types"
	"event-pool/validators"
	"google.golang.org/grpc"
)

const (
	IbftKeyName   = "validator.key"
	KeyEpochSize  = "epochSize"
	MinimumHeight = 2
	ibftProto     = "/ibft/0.2"
)

var (
	ErrProposerSealByNonValidator   = errors.New("proposer seal by non-validator")
	ErrInvalidMixHash               = errors.New("invalid mixhash")
	ErrInvalidSha3Uncles            = errors.New("invalid sha3 uncles")
	ErrWrongDifficulty              = errors.New("wrong difficulty")
	ErrParentCommittedSealsNotFound = errors.New("parent committed seals not found")
)

type forkManagerInterface interface {
	Initialize() error
	Close() error
	GetSigner(uint64) (signer.Signer, error)
	GetValidatorStore(uint64) (fork.ValidatorStore, error)
	GetValidators(uint64) (validators.Validators, error)
	GetHooks(uint64) fork.HooksInterface
}

// backendIBFT represents the IBFT consensus mechanism object
type backendIBFT struct {
	consensus *IBFTConsensus

	// Static References
	logger         *zap.SugaredLogger     // Reference to the logging
	blockchain     *blockchain.Blockchain // Reference to the blockchain layer
	network        *network.Server        // Reference to the networking layer
	syncer         syncer.Syncer          // Reference to the sync protocol
	secretsManager secrets.SecretsManager // Reference to the secret manager
	Grpc           *grpc.Server           // Reference to the gRPC manager
	operator       *operator              // Reference to the gRPC service of IBFT
	transport      transport              // Reference to the transport protocol
	metrics        *consensus.Metrics     // Reference to the metrics service

	// Dynamic References
	forkManager       forkManagerInterface  // Manager to hold IBFT Forks
	currentSigner     signer.Signer         // Signer at current sequence
	currentValidators validators.Validators // validator at current sequence
	currentHooks      fork.HooksInterface   // Hooks at current sequence

	// Configurations
	config              *consensus.Config // Consensus configuration
	epochSize           uint64
	quorumSizeBlockNum  uint64
	blockTime           time.Duration // Minimum block generation time in seconds
	additionalEpochTime time.Duration // an additional time of block generation at epoch

	// Channels
	closeCh chan struct{} // Channel for closing
}

// Factory implements the base consensus Factory method
func Factory(params *consensus.Params) (consensus.Consensus, error) {
	// defaults for user set fields in genesis
	var (
		epochSize          = uint64(blockchain.DefaultEpochSize)
		quorumSizeBlockNum = uint64(0)
	)

	if definedEpochSize, ok := params.Config.Config[KeyEpochSize]; ok {
		// Epoch size is defined, use the passed in one
		readSize, ok := definedEpochSize.(float64)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		epochSize = uint64(readSize)
	}

	if rawBlockNum, ok := params.Config.Config["quorumSizeBlockNum"]; ok {
		// Block number specified for quorum size switch
		readBlockNum, ok := rawBlockNum.(float64)
		if !ok {
			return nil, errors.New("invalid type assertion")
		}

		quorumSizeBlockNum = uint64(readBlockNum)
	}

	logger := params.Logger.Named("ibft")
	// logger := params.Logger

	forkManager, err := fork.NewForkManager(
		logger,
		params.Blockchain,
		params.SecretsManager,
		params.Config.Path,
		epochSize,
		params.Config.Config,
	)

	if err != nil {
		return nil, err
	}

	p := &backendIBFT{
		// References
		logger:     logger,
		blockchain: params.Blockchain,
		network:    params.Network,
		syncer: syncer.NewSyncer(
			params.Logger,
			params.Network,
			params.Blockchain,
			time.Duration(params.BlockTime)*3*time.Second,
		),
		secretsManager: params.SecretsManager,
		Grpc:           params.Grpc,
		metrics:        params.Metrics,
		forkManager:    forkManager,

		// Configurations
		config:             params.Config,
		epochSize:          epochSize,
		quorumSizeBlockNum: quorumSizeBlockNum,
		blockTime:          time.Duration(params.BlockTime) * time.Second,

		// Channels
		closeCh: make(chan struct{}),
	}

	// Istanbul requires a different header hash function
	p.SetHeaderHash()

	return p, nil
}

func (i *backendIBFT) Initialize() error {
	// register the grpc operator
	if i.Grpc != nil {
		i.operator = &operator{ibft: i}
		proto.RegisterIbftOperatorServer(i.Grpc, i.operator)
	}

	// start the transport protocol
	if err := i.setupTransport(); err != nil {
		return err
	}

	// initialize fork manager
	if err := i.forkManager.Initialize(); err != nil {
		return err
	}

	if err := i.updateCurrentModules(i.blockchain.Header().Number + 1); err != nil {
		return err
	}

	i.logger.Info("validator key", "addr", i.currentSigner.Address().String())

	i.consensus = newIBFT(
		i.logger.Named("consensus"),
		i,
		i,
		"0.1.0",
	)

	// Ensure consensus takes into account user configured block production time
	i.consensus.ExtendRoundTimeout(i.blockTime)
	return nil
}

// sync runs the syncer in the background to receive blocks from advanced peers
func (i *backendIBFT) startSyncing() {
	callInsertBlockHook := func(block *types.Block) bool {
		if err := i.currentHooks.PostInsertBlock(block); err != nil {
			i.logger.Error("failed to call PostInsertBlock", "height", block.Header.Number, "error", err)
		}

		if err := i.updateCurrentModules(block.Number() + 1); err != nil {
			i.logger.Error("failed to update sub modules", "height", block.Number()+1, "err", err)
		}

		return false
	}

	if err := i.syncer.Sync(
		callInsertBlockHook,
	); err != nil {
		i.logger.Error("watch sync failed", "err", err)
	}

}

// Start starts the IBFT consensus
func (i *backendIBFT) Start() error {
	// Start the syncer
	if err := i.syncer.Start(); err != nil {
		return err
	}

	// Start syncing blocks from other peers
	go i.startSyncing()

	// Start the actual consensus protocol
	go i.startConsensus()

	return nil
}

// GetSyncProgression gets the latest sync progression, if any
func (i *backendIBFT) GetSyncProgression() *progress.Progression {
	return i.syncer.GetSyncProgression()
}

func (i *backendIBFT) startConsensus() {
	var (
		newBlockSub   = i.blockchain.SubscribeEvents()
		syncerBlockCh = make(chan struct{})
	)

	// Receive a notification every time syncer manages
	// to insert a valid block. Used for cancelling active consensus
	// rounds for a specific height
	go func() {
		eventCh := newBlockSub.GetEventCh()

		for {
			if ev := <-eventCh; ev.Source == "syncer" {
				if ev.NewChain[0].Number < i.blockchain.Header().Number {
					// The blockchain notification system can eventually deliver
					// stale block notifications. These should be ignored
					continue
				}

				syncerBlockCh <- struct{}{}
			}
		}
	}()

	defer newBlockSub.Close()

	var (
		sequenceCh  = make(<-chan struct{})
		isValidator bool
	)

	for {
		var (
			latest  = i.blockchain.Header().Number
			pending = latest + 1
		)

		if err := i.updateCurrentModules(pending); err != nil {
			i.logger.Errorw(
				"failed to update submodules",
				"height", pending,
				"err", err,
			)
		}

		// Update the No.of validator metric
		i.metrics.Validators.Set(float64(i.currentValidators.Len()))
		isValidator = i.IsActiveValidator()

		if isValidator {
			sequenceCh = i.consensus.runSequence(pending)
		}

		select {
		case <-syncerBlockCh:
			if isValidator {
				i.consensus.stopSequence()
				i.logger.Info("canceled sequence", "sequence", pending)
			}
		case <-sequenceCh:
		case <-i.closeCh:
			if isValidator {
				i.consensus.stopSequence()
			}

			return
		}
	}
}

// isActiveValidator returns whether my signer belongs to current validators
// func (i *backendIBFT) isActiveValidator() bool {
// 	return i.currentValidators.Includes(i.currentSigner.Address())
// }

// IsActiveValidator returns whether my signer belongs to specific validator
func (i *backendIBFT) IsActiveValidator() bool {
	return i.currentValidators.Includes(i.currentSigner.Address())
}

// IsEpochHeight returns whether the current height is at epoch block
func (i *backendIBFT) IsEpochHeight(height uint64) bool {
	return height > 0 && height%i.epochSize == 0
}

// updateMetrics will update various metrics based on the given block
// currently we capture No.of Txs and block interval metrics using this function
func (i *backendIBFT) updateMetrics(block *types.Block) {
	// get previous header
	prvHeader, _ := i.blockchain.GetHeaderByNumber(block.Number() - 1)
	parentTime := time.Unix(int64(prvHeader.Timestamp), 0)
	headerTime := time.Unix(int64(block.Header.Timestamp), 0)

	// Update the block interval metric
	if block.Number() > 1 {
		i.metrics.EventInterval.Set(
			headerTime.Sub(parentTime).Seconds(),
		)
	}
}

// verifyHeaderImpl verifies fields including Extra
// for the past or being proposed header
func (i *backendIBFT) verifyHeaderImpl(
	parent, header *types.Header,
	headerSigner signer.Signer,
	validators validators.Validators,
	hooks fork.HooksInterface,
	shouldVerifyParentCommittedSeals bool,
) error {
	// ensure the extra data is correctly formatted
	if _, err := headerSigner.GetIBFTExtra(header); err != nil {
		return err
	}

	// verify the ProposerSeal
	if err := verifyProposerSeal(
		header,
		headerSigner,
		validators,
	); err != nil {
		return err
	}

	// verify the ParentCommittedSeals
	if err := i.verifyParentCommittedSeals(
		parent, header,
		shouldVerifyParentCommittedSeals,
	); err != nil {
		return err
	}

	// Additional header verification
	if err := hooks.VerifyHeader(header); err != nil {
		return err
	}

	return nil
}

// VerifyHeader wrapper for verifying headers
func (i *backendIBFT) VerifyHeader(header *types.Header) error {
	parent, ok := i.blockchain.GetHeaderByNumber(header.Number - 1)
	if !ok {
		return fmt.Errorf(
			"unable to get parent header for block number %d",
			header.Number,
		)
	}

	headerSigner, validatorsSet, hooks, err := getModulesFromForkManager(
		i.forkManager,
		header.Number,
	)
	if err != nil {
		return err
	}

	// verify all the header fields
	if err := i.verifyHeaderImpl(
		parent,
		header,
		headerSigner,
		validatorsSet,
		hooks,
		false,
	); err != nil {
		return err
	}

	// verify the Committed Seals
	// CommittedSeals exists only in the finalized header
	if err := headerSigner.VerifyCommittedSeals(
		header,
		validatorsSet,
		i.quorumSize(header.Number)(validatorsSet),
	); err != nil {
		return err
	}

	return nil
}

// quorumSize returns a callback that when executed on a Validators computes
// number of votes required to reach quorum based on the size of the set.
// The blockNumber argument indicates which formula was used to calculate the result (see PRs #513, #549)
func (i *backendIBFT) quorumSize(blockNumber uint64) QuorumImplementation {
	if blockNumber < i.quorumSizeBlockNum {
		return LegacyQuorumSize
	}

	return OptimalQuorumSize
}

// ProcessHeaders updates the snapshot based on previously verified headers
func (i *backendIBFT) ProcessHeaders(headers []*types.Header) error {
	for _, header := range headers {
		hooks := i.forkManager.GetHooks(header.Number)

		if err := hooks.ProcessHeader(header); err != nil {
			return err
		}
	}

	return nil
}

// GetBlockCreator retrieves the block signer from the extra data field
func (i *backendIBFT) GetBlockCreator(header *types.Header) (types.Address, error) {
	signerInfo, err := i.forkManager.GetSigner(header.Number)
	if err != nil {
		return types.ZeroAddress, err
	}

	return signerInfo.EcrecoverFromHeader(header)
}

// GetEpoch returns the current epoch
func (i *backendIBFT) GetEpoch(number uint64) uint64 {
	if number%i.epochSize == 0 {
		return number / i.epochSize
	}

	return number/i.epochSize + 1
}

// Close closes the IBFT consensus mechanism, and does write back to disk
func (i *backendIBFT) Close() error {
	close(i.closeCh)

	if i.syncer != nil {
		if err := i.syncer.Close(); err != nil {
			return err
		}
	}

	if i.forkManager != nil {
		if err := i.forkManager.Close(); err != nil {
			return err
		}
	}

	return nil
}

// SetHeaderHash updates hash calculation function for IBFT
func (i *backendIBFT) SetHeaderHash() {
	types.HeaderHash = func(h *types.Header) types.Hash {
		signer, err := i.forkManager.GetSigner(h.Number)
		if err != nil {
			return types.ZeroHash
		}

		hash, err := signer.CalculateHeaderHash(h)
		if err != nil {
			return types.ZeroHash
		}

		return hash
	}
}

// updateCurrentModules updates Signer, Hooks, and Validators
// that are used at specified height
// by fetching from ForkManager
func (i *backendIBFT) updateCurrentModules(height uint64) error {
	lastSigner := i.currentSigner

	signerInfo, validatorsSet, hooks, err := getModulesFromForkManager(i.forkManager, height)
	if err != nil {
		return err
	}

	i.currentSigner = signerInfo
	i.currentValidators = validatorsSet
	i.currentHooks = hooks
	i.logFork(lastSigner, signerInfo)

	return nil
}

// logFork logs validation type switch
func (i *backendIBFT) logFork(
	lastSigner, signer signer.Signer,
) {
	if lastSigner != nil && signer != nil && lastSigner.Type() != signer.Type() {
		i.logger.Info("IBFT validation type switched", "old", lastSigner.Type(), "new", signer.Type())
	}
}

func (i *backendIBFT) verifyParentCommittedSeals(
	parent, header *types.Header,
	shouldVerifyParentCommittedSeals bool,
) error {
	parentSigner, parentValidators, _, err := getModulesFromForkManager(
		i.forkManager,
		parent.Number,
	)

	if err != nil {
		return err
	}

	// if shouldVerifyParentCommittedSeals is false, skip the verification
	// when header doesn't have Parent Committed Seals (Backward Compatibility)
	return parentSigner.VerifyParentCommittedSeals(
		parent,
		header,
		parentValidators,
		i.quorumSize(parent.Number)(parentValidators),
		shouldVerifyParentCommittedSeals,
	)
}

// getModulesFromForkManager is a helper function to get all modules from ForkManager
func getModulesFromForkManager(forkManager forkManagerInterface, height uint64) (
	signer.Signer,
	validators.Validators,
	fork.HooksInterface,
	error,
) {
	signerInfo, err := forkManager.GetSigner(height)
	if err != nil {
		return nil, nil, nil, err
	}

	validatorsSet, err := forkManager.GetValidators(height)
	if err != nil {
		return nil, nil, nil, err
	}

	hooks := forkManager.GetHooks(height)

	return signerInfo, validatorsSet, hooks, nil
}

// verifyProposerSeal verifies ProposerSeal in IBFT Extra of header
// and make sure signer belongs to validators and validators subset
func verifyProposerSeal(
	header *types.Header,
	signer signer.Signer,
	validators validators.Validators,
) error {
	proposer, err := signer.EcrecoverFromHeader(header)
	if err != nil {
		return err
	}

	proposerInVal := validators.Includes(proposer)
	//color.Yellow("proposerInVal %v proposerInSubVal %v", proposerInVal, proposerInSubVal)
	if !proposerInVal {
		return ErrProposerSealByNonValidator
	}

	return nil
}
