package ibft

import (
	"fmt"
	"math"
	"time"

	"event-pool/consensus"
	"event-pool/consensus/ibft/signer"
	"event-pool/helper/hex"
	"event-pool/ibft/messages"
	"event-pool/types"
)

func (i *backendIBFT) BuildProposal(blockNumber uint64) []byte {
	var (
		latestHeader = i.blockchain.Header()
	)

	now := time.Now()

	block, err := i.buildBlock(latestHeader)
	if err != nil {
		i.logger.Error("cannot build block", "num", blockNumber, "err", err)

		return nil
	}

	i.logger.Debug("building block time", "height", blockNumber, "t", time.Since(now))

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
		i.logger.Error("cannot write block: corrupted extra data",
			"err", err,
			"committedSeals", committedSealsStr)

		return
	}

	newBlock.Header = header

	// Save the block locally
	if err := i.blockchain.WriteBlock(newBlock, "consensus"); err != nil {
		i.logger.Error("cannot write block", "err", err)

		return
	}

	i.updateMetrics(newBlock)

	i.logger.Info(
		"block committed",
		"number", newBlock.Number(),
		"hash", newBlock.Hash(),
		"validation_type", i.currentSigner.Type(),
		"committed", len(committedSeals),
	)

	if err := i.currentHooks.PostInsertBlock(newBlock); err != nil {
		i.logger.Error(
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
		i.logger.Error(
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

	// build the block
	block := consensus.BuildBlock(consensus.BuildBlockParams{
		Header: header,
	})

	// write the seal of the block after all the fields are completed
	header, err = i.currentSigner.WriteProposerSeal(header)
	if err != nil {
		return nil, err
	}

	block.Header = header

	// compute the hash, this is only a provisional hash since the final one
	// is sealed after all the committed seals
	block.Header.ComputeHash()

	i.logger.Info("build block", "number", header.Number)

	return block, nil
}

// NewHeader generates current header. It must be separated so the preparation steps before round starts can generate its header
func (i *backendIBFT) NewHeader(parent *types.Header) (*types.Header, error) {
	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
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
