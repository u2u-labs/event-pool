package blockchain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	"event-pool/prisma/db"
	"event-pool/validators"
	"go.uber.org/zap"

	"event-pool/chain"
	"event-pool/types"
	lru "github.com/hashicorp/golang-lru"
)

const (
	defaultCacheSize int = 100 // The default size for Blockchain LRU cache structures
)

var (
	ErrNoBlock              = errors.New("no block data passed in")
	ErrParentNotFound       = errors.New("parent block not found")
	ErrInvalidParentHash    = errors.New("parent block hash is invalid")
	ErrParentHashMismatch   = errors.New("invalid parent block hash")
	ErrInvalidBlockSequence = errors.New("invalid block sequence")
	ErrInvalidSha3Uncles    = errors.New("invalid block sha3 uncles root")
	ErrInvalidTxRoot        = errors.New("invalid block transactions root")
	ErrInvalidStateRoot     = errors.New("invalid block state root")
)

// Blockchain is a blockchain reference
type Blockchain struct {
	logger *zap.SugaredLogger // The logger object

	db        *db.PrismaClient
	consensus Verifier
	executor  Executor

	config  *chain.NodeChain // Config containing chain information
	genesis types.Hash       // The hash of the genesis block

	headersCache *lru.Cache // LRU cache for the headers

	currentHeader atomic.Value // The current header

	stream *eventStream // Event subscriptions

	writeLock sync.Mutex
}

type Verifier interface {
	VerifyHeader(header *types.Header) error
	ProcessHeaders(headers []*types.Header) error
	GetBlockCreator(header *types.Header) (types.Address, error)
	//PreCommitState(header *types.Header, txn *state.Transition) error
}

type Executor interface {
	//ProcessBlock(parentRoot types.Hash, block *types.Block, blockCreator types.Address, isInserted bool) (*state.Transition, error)
	//BeginTxn(parentRoot types.Hash, header *types.Header, coinbaseReceiver types.Address) (*state.Transition, error)
}

type BlockResult struct {
	Root   types.Hash
	logger *zap.SugaredLogger
}

// NewBlockchain creates a new blockchain object
func NewBlockchain(
	logger *zap.SugaredLogger,
	config *chain.NodeChain,
	consensus Verifier,
	executor Executor,
) (*Blockchain, error) {
	b := &Blockchain{
		logger:    logger.Named("blockchain"),
		config:    config,
		consensus: consensus,
		executor:  executor,
		stream:    &eventStream{},
	}

	if err := b.initCaches(defaultCacheSize); err != nil {
		return nil, err
	}

	// Push the initial event to the stream
	b.stream.push(&Event{})

	return b, nil
}

// initCaches initializes the blockchain caches with the specified size
func (b *Blockchain) initCaches(size int) error {
	var err error

	b.headersCache, err = lru.New(size)
	if err != nil {
		return fmt.Errorf("unable to create headers cache, %w", err)
	}

	return nil
}

func (b *Blockchain) GetConsensus() Verifier {
	return b.consensus
}

// SetConsensus sets the consensus
func (b *Blockchain) SetConsensus(c Verifier) {
	b.consensus = c
}

// setCurrentHeader sets the current header
func (b *Blockchain) setCurrentHeader(h *types.Header, diff *big.Int) {
	// Update the header (atomic)
	header := h.Copy()
	b.currentHeader.Store(header)
}

// Header returns the current header (atomic)
func (b *Blockchain) Header() *types.Header {
	header, ok := b.currentHeader.Load().(*types.Header)
	if !ok {
		return nil
	}

	return header
}

// Config returns the blockchain configuration
func (b *Blockchain) Config() *chain.Params {
	return b.config.Params
}

// VerifyPotentialBlock does the minimal block verification without consulting the
// consensus layer. Should only be used if consensus checks are done
// outside the method call
func (b *Blockchain) VerifyPotentialBlock(block *types.Block, currentValidators validators.Validators) error {
	// Do just the initial block verification
	return b.verifyBlock(block)
}

// VerifyFinalizedBlock verifies that the block is valid by performing a series of checks.
// It is assumed that the block status is sealed (committed)
func (b *Blockchain) VerifyFinalizedBlock(block *types.Block) error {
	// Make sure the consensus layer verifies this block header
	if err := b.consensus.VerifyHeader(block.Header); err != nil {
		return fmt.Errorf("failed to verify the header: %w", err)
	}

	// Do the initial block verification
	if err := b.verifyBlock(block); err != nil {
		return err
	}

	return nil
}

// verifyBlock does the base (common) block verification steps by
// verifying the block body as well as the parent information
func (b *Blockchain) verifyBlock(block *types.Block) error {
	// Make sure the block is present
	if block == nil {
		return ErrNoBlock
	}

	// Make sure the block is in line with the parent block
	if err := b.verifyBlockParent(block); err != nil {
		return err
	}

	// Make sure the block body data is valid
	if err := b.verifyBlockBody(block); err != nil {
		return err
	}
	return nil
}

// verifyBlockParent makes sure that the child block is in line
// with the locally saved parent block. This means checking:
// - The parent exists
// - The hashes match up
// - The block numbers match up
func (b *Blockchain) verifyBlockParent(childBlock *types.Block) error {
	// TODO: This needs to be updated to use the new header verification
	return nil
}

// verifyBlockBody verifies that the block body is valid. This means checking:
// - The trie roots match up (state, transactions, receipts, uncles)
// - The receipts match up
// - The execution result matches up
func (b *Blockchain) verifyBlockBody(block *types.Block) error {
	// Execute the transactions in the block and grab the result
	blockResult, executeErr := b.executeBlockLogs(block)
	if executeErr != nil {
		return fmt.Errorf("unable to execute block transactions, %w", executeErr)
	}

	// Verify the local execution result with the proposed block data
	if err := blockResult.verifyBlockResult(block); err != nil {
		return fmt.Errorf("[verifyBlockResult] unable to verify block execution result, %w", err)
	}
	return nil
}

// verifyBlockResult verifies that the block transaction execution result
// matches up to the expected values
func (br *BlockResult) verifyBlockResult(referenceBlock *types.Block) error {
	if br.Root != referenceBlock.Header.Hash {
		// This log message is used to report a mismatch between the block result root and the reference block state root.
		// The message includes the block number, the expected state root, and the received state root.
		br.logger.Error(fmt.Sprintf(
			"state hash hash mismatch: have %s, want %s",
			br.Root,
			referenceBlock.Header.Hash,
		))
		return ErrInvalidStateRoot
	}

	return nil
}

// executeBlockLogs executes the transactions in the block locally,
// and reports back the block execution result
func (b *Blockchain) executeBlockLogs(block *types.Block) (*BlockResult, error) {

	return &BlockResult{}, nil
}

// WriteBlock writes a single block to the local blockchain.
// It doesn't do any kind of verification, only commits the block to the DB
func (b *Blockchain) WriteBlock(block *types.Block, source string) error {
	b.writeLock.Lock()
	defer b.writeLock.Unlock()

	if block.Number() <= b.Header().Number() {
		b.logger.Info("block already inserted", "block", block.Number(), "source", source)

		return nil
	}

	header := block.Header

	// Write the header to the chain
	evnt := &Event{Source: source}

	// update snapshot
	if err := b.consensus.ProcessHeaders([]*types.Header{header}); err != nil {
		return err
	}

	b.dispatchEvent(evnt)

	logArgs := []any{
		"number", header.Number(),
		"hash", header.Hash,
		"parent", header.ParentHash,
	}

	b.logger.Infow("new block", logArgs...)

	return nil
}

// dispatchEvent pushes a new event to the stream
func (b *Blockchain) dispatchEvent(evnt *Event) {
	b.stream.push(evnt)
}

// Close closes the DB connection
func (b *Blockchain) Close() error {
	return b.db.Disconnect()
}
