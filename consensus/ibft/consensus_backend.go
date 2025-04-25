package ibft

import (
	"fmt"
	"math"
	"time"

	"event-pool/consensus"
	"event-pool/consensus/ibft/signer"
	"event-pool/helper/common"
	"event-pool/helper/hex"
	"event-pool/ibft/messages"
	"event-pool/state"
	"event-pool/types"
)

func (i *backendIBFT) BuildProposal(blockNumber uint64) []byte {
	var (
		latestHeader = i.blockchain.Header()
	)

	block, err := i.buildBlock(latestHeader)
	if err != nil {
		i.logger.Errorw("cannot build block", "num", blockNumber, "err", err)

		return nil
	}

	return block.MarshalRLP()
}

func (i *backendIBFT) InsertBlock(
	proposal []byte,
	committedSeals []*messages.CommittedSeal,
) {
	newBlock := &types.Block{}
	if err := newBlock.UnmarshalRLP(proposal); err != nil {
		i.logger.Error("cannot unmarshal proposal", "err", err)

		return
	}

	committedSealsMap := make(map[types.Address][]byte, len(committedSeals))

	for _, cm := range committedSeals {
		committedSealsMap[types.BytesToAddress(cm.Signer)] = cm.Signature
	}

	// Push the committed seals to the header
	header, err := i.currentSigner.WriteCommittedSeals(newBlock.Header, committedSealsMap)
	if err != nil {
		i.logger.Error("cannot write committed seals", "err", err)

		return
	}

	// WriteCommittedSeals alters the extra data before writing the block
	// It doesn't handle errors while pushing changes which can result in
	// corrupted extra data.
	// We don't know exact circumstance of the unmarshalRLP error
	// This is a safety net to help us narrow down and also recover before
	// writing the block
	if err := i.ValidateExtraDataFormat(newBlock.Header); err != nil {
		//Format committed seals to make them more readable
		committedSealsStr := make([]string, len(committedSealsMap))
		for i, seal := range committedSeals {
			committedSealsStr[i] = fmt.Sprintf("{signer=%v signature=%v}",
				hex.EncodeToHex(seal.Signer),
				hex.EncodeToHex(seal.Signature))
		}
		i.logger.Errorw("cannot write block: corrupted extra data",
			"err", err,
			"committedSeals", committedSealsStr)

		return
	}

	newBlock.Header = header

	// Save the block locally
	if err := i.blockchain.WriteBlock(newBlock, "consensus"); err != nil {
		i.logger.Errorw("cannot write block", "err", err)

		return
	}

	i.updateMetrics(newBlock)

	i.logger.Infow(
		"block committed",
		"number", newBlock.Number(),
		"hash", newBlock.Hash(),
		"validation_type", i.currentSigner.Type(),
		"committed", len(committedSeals),
	)

	if err := i.currentHooks.PostInsertBlock(newBlock); err != nil {
		i.logger.Errorw(
			"failed to call PostInsertBlock hook",
			"height", newBlock.Number(),
			"hash", newBlock.Hash(),
			"err", err,
		)

		return
	}
}

func (i *backendIBFT) ID() []byte {
	return i.currentSigner.Address().Bytes()
}

func (i *backendIBFT) MaximumFaultyNodes() uint64 {
	return uint64(CalcMaxFaultyNodes(i.currentValidators))
}

func (i *backendIBFT) Quorum(blockNumber uint64) uint64 {
	validators, err := i.forkManager.GetValidators(blockNumber)
	if err != nil {
		i.logger.Errorw(
			"failed to get validators when calculation quorum",
			"height", blockNumber,
			"err", err,
		)

		// return Math.MaxInt32 to prevent overflow when casting to int in go-ibft package
		return math.MaxInt32
	}

	quorumFn := i.quorumSize(blockNumber)

	return uint64(quorumFn(validators))
}

// buildBlock builds the block, based on the passed in snapshot and parent header
func (i *backendIBFT) buildBlock(parent *types.Header) (*types.Block, error) {
	header, err := i.NewHeader(parent)
	if err != nil {
		return nil, err
	}

	if err := i.currentHooks.ModifyHeader(header, i.currentSigner.Address()); err != nil {
		return nil, err
	}

	parentCommittedSeals, err := i.extractParentCommittedSeals(parent)
	if err != nil {
		return nil, err
	}

	i.currentSigner.InitIBFTExtra(header, i.currentValidators, parentCommittedSeals)

	i.logger.Debugw("begin", "stateRoot", parent.StateRoot, "number", header.Number)
	transition, err := i.executor.BeginTxn(parent.StateRoot, header, i.currentSigner.Address())
	if err != nil {
		return nil, err
	}

	txs := i.writeTransactions(math.MaxUint64, header.Number, transition)

	// build the block
	block := consensus.BuildBlock(consensus.BuildBlockParams{
		Header: header,
		Txns:   txs,
	})
	if len(txs) > 0 {
		i.logger.Debugw("build block", "txs", len(txs), "blockHash", block.Hash())
	}

	_, root := transition.Commit()
	header.StateRoot = root
	i.logger.Debugw("finish", "stateRoot", header.StateRoot, "number", header.Number)

	// write the seal of the block after all the fields are completed
	header, err = i.currentSigner.WriteProposerSeal(header)
	if err != nil {
		return nil, err
	}

	block.Header = header

	// compute the hash, this is only a provisional hash since the final one
	// is sealed after all the committed seals
	block.Header.ComputeHash()

	i.logger.Infow("build block", "number", header.Number)

	return block, nil
}

// NewHeader generates current header. It must be separated so the preparation steps before round starts can generate its header
func (i *backendIBFT) NewHeader(parent *types.Header) (*types.Header, error) {
	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		ChainId:    parent.ChainId,
		SideHead:   parent.SideHead,
	}

	// get side head
	if header.Number%common.SideHeadSyncInterval == 0 || header.Number <= 1 {
		sideHead, err := i.blockchain.GetEthereumClient().GetLatestBlock()
		if err != nil {
			return nil, err
		}
		header.SideHead = sideHead
	}

	return header, nil
}

// extractCommittedSeals extracts CommittedSeals from header
func (i *backendIBFT) extractCommittedSeals(
	header *types.Header,
) (signer.Seals, error) {
	signerInfo, err := i.forkManager.GetSigner(header.Number)
	if err != nil {
		return nil, err
	}

	extra, err := signerInfo.GetIBFTExtra(header)
	if err != nil {
		return nil, err
	}

	return extra.CommittedSeals, nil
}

// extractParentCommittedSeals extracts ParentCommittedSeals from header
func (i *backendIBFT) extractParentCommittedSeals(
	header *types.Header,
) (signer.Seals, error) {
	if header.Number == 0 {
		return nil, nil
	}

	return i.extractCommittedSeals(header)
}

// ValidateExtraDataFormat Verifies that extra data can be unmarshaled
func (i *backendIBFT) ValidateExtraDataFormat(header *types.Header) error {
	blockSigner, _, _, err := getModulesFromForkManager(i.forkManager, header.Number)

	if err != nil {
		return err
	}
	_, err = blockSigner.GetIBFTExtra(header)

	return err
}

// ----------------------------------------------------------------------------

type status uint8

const (
	success status = iota
	fail
	skip
)

type txExeResult struct {
	tx     *types.Transaction
	status status
}

type transitionInterface interface {
	Write(txn *types.Transaction) error
	WriteFailedReceipt(txn *types.Transaction) error
}

func (i *backendIBFT) writeTransactions(
	gasLimit,
	blockNumber uint64,
	transition transitionInterface,
) (executed []*types.Transaction) {
	executed = make([]*types.Transaction, 0)
	blockTimer := time.NewTimer(i.blockTime)

	if !i.currentHooks.ShouldWriteTransactions(blockNumber) {
		// wait for the timer to expire
		<-blockTimer.C
		return
	}

	var (
		successful = 0
		failed     = 0
		skipped    = 0
	)

	defer func() {
		i.logger.Infow(
			"executed txs",
			"successful", successful,
			"failed", failed,
			"skipped", skipped,
			"remaining", i.txpool.Length(),
		)
	}()

	i.txpool.Prepare()

write:
	for {
		select {
		case <-blockTimer.C:
			return
		default:
			// execute transactions one by one
			result, ok := i.writeTransaction(
				i.txpool.Peek(),
				transition,
				gasLimit,
			)

			if !ok {
				break write
			}

			tx := result.tx

			switch result.status {
			case success:
				executed = append(executed, tx)
				successful++
			case fail:
				failed++
			case skip:
				skipped++
			}
		}
	}

	//	wait for the timer to expire
	<-blockTimer.C

	return
}

func (i *backendIBFT) writeTransaction(
	tx *types.Transaction,
	transition transitionInterface,
	gasLimit uint64,
) (*txExeResult, bool) {
	if tx == nil {
		return nil, false
	}

	if tx.ExceedsBlockGasLimit(gasLimit) {
		i.txpool.Drop(tx)

		if err := transition.WriteFailedReceipt(tx); err != nil {
			i.logger.Error(
				fmt.Sprintf(
					"unable to write failed receipt for transaction %s",
					tx.Hash,
				),
			)
		}

		// continue processing
		return &txExeResult{tx, fail}, true
	}

	if err := transition.Write(tx); err != nil {
		if appErr, ok := err.(*state.TransitionApplicationError); ok && appErr.IsRecoverable { //nolint:errorlint
			i.txpool.Demote(tx)

			return &txExeResult{tx, skip}, true
		} else {
			i.txpool.Drop(tx)

			return &txExeResult{tx, fail}, true
		}
	}

	i.txpool.Pop(tx)

	return &txExeResult{tx, success}, true
}
