package blockchain

import (
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"
	"sync/atomic"

	"event-pool/blockchain/storage"
	"event-pool/blockchain/storage/leveldb"
	"event-pool/blockchain/storage/memory"
	"event-pool/blockchain/storage/prismadb"
	db2 "event-pool/internal/db"
	"event-pool/internal/monitor"
	"event-pool/pkg/ethereum"
	"event-pool/state"
	"event-pool/validators"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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
	ErrInvalidTxRoot        = errors.New("invalid block transactions root")
	ErrInvalidStateRoot     = errors.New("invalid block state root")
)

// Blockchain is a blockchain reference
type Blockchain struct {
	logger *zap.SugaredLogger // The logger object

	db        storage.Storage // The database object
	consensus Verifier
	executor  Executor
	txSigner  TxSigner

	config  *chain.NodeChain // Config containing chain information
	genesis types.Hash       // The hash of the genesis block

	headersCache *lru.Cache // LRU cache for the headers

	currentHeader atomic.Value // The current header

	stream *eventStream // Event subscriptions

	rpcClient          *ethereum.Client
	nodeStorageAddress types.Address
	monitor            *monitor.Monitor

	writeLock sync.Mutex
}

type Verifier interface {
	VerifyHeader(header *types.Header) error
	ProcessHeaders(headers []*types.Header) error
	GetBlockCreator(header *types.Header) (types.Address, error)
}

type Executor interface {
	ProcessBlock(
		parentRoot types.Hash,
		block *types.Block,
		blockCreator types.Address,
	) (*state.Transition, error)
}

type BlockResult struct {
	Root   types.Hash
	logger *zap.SugaredLogger
}

type TxSigner interface {
	// Sender returns the sender of the transaction
	Sender(tx *types.Transaction) (types.Address, error)
}

// NewBlockchain creates a new blockchain object
func NewBlockchain(
	logger *zap.SugaredLogger,
	dataDir string,
	config *chain.NodeChain,
	consensus Verifier,
	executor Executor,
	txSigner TxSigner,
) (*Blockchain, error) {
	b := &Blockchain{
		logger:    logger.Named("blockchain"),
		config:    config,
		consensus: consensus,
		executor:  executor,
		stream:    &eventStream{},
		txSigner:  txSigner,
	}

	var (
		db  storage.Storage
		err error
	)

	// Initialize database
	dbClient, err := db2.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if db, err = prismadb.NewSQLStorage(
		logger,
		dbClient,
	); err != nil {
		return nil, err
	}
	if dataDir == "" {
		if db, err = memory.NewMemoryStorage(nil); err != nil {
			return nil, err
		}
	} else {
		if db, err = leveldb.NewLevelDBStorage(
			filepath.Join(dataDir, "blockchain"),
			logger,
		); err != nil {
			return nil, err
		}
	}

	b.db = db

	client, err := ethereum.NewClient(config.RpcInfo.RpcUrl, b.config.Params.ChainID, int(config.RpcInfo.BlockTime), dbClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ethereum client for chain %d: %w", b.config.Params.ChainID, err)
	}
	b.rpcClient = client
	b.nodeStorageAddress = config.NodeStorageAddress
	ethClients := make(map[int]*ethereum.Client)
	ethClients[b.config.Params.ChainID] = client
	mon := monitor.NewMonitor(ethClients, dbClient, nil)
	b.monitor = mon

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

// ComputeGenesis computes the genesis hash, and updates the blockchain reference
func (b *Blockchain) ComputeGenesis() error {
	// try to write the genesis block
	head, ok := b.db.ReadHeadHash()

	if ok {
		// initialized storage
		b.genesis, ok = b.db.ReadCanonicalHash(0)
		if !ok {
			return fmt.Errorf("failed to load genesis hash")
		}

		// validate that the genesis file in storage matches the chain.Genesis
		if b.genesis != b.config.Genesis.StateRoot {
			return fmt.Errorf("genesis file does not match current genesis")
		}

		header, ok := b.GetHeaderByHash(head)
		if !ok {
			return fmt.Errorf("failed to get header with hash %s", head.String())
		}

		b.logger.Infow(
			"Current header",
			"hash",
			header.Hash.String(),
			"number",
			header.Number,
		)

		b.setCurrentHeader(header)
	} else {
		// empty storage, write the genesis
		if err := b.writeGenesis(b.config.Genesis); err != nil {
			return err
		}
	}

	b.logger.Infow("genesis", "hash", b.config.Genesis.StateRoot)

	return nil
}

func (b *Blockchain) GetConsensus() Verifier {
	return b.consensus
}

// GetHeaderByHash returns the header by his hash
func (b *Blockchain) GetHeaderByHash(hash types.Hash) (*types.Header, bool) {
	return b.readHeader(hash)
}

// SetConsensus sets the consensus
func (b *Blockchain) SetConsensus(c Verifier) {
	b.consensus = c
}

// setCurrentHeader sets the current header
func (b *Blockchain) setCurrentHeader(h *types.Header) {
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

// executeBlockTransactions executes the transactions in the block locally,
// and reports back the block execution result
func (b *Blockchain) executeBlockTransactions(block *types.Block) (*BlockResult, error) {
	header := block.Header

	parent, ok := b.readHeader(header.ParentHash)
	if !ok {
		return nil, ErrParentNotFound
	}

	blockCreator, err := b.consensus.GetBlockCreator(header)
	if err != nil {
		return nil, err
	}

	txn, err := b.executor.ProcessBlock(parent.StateRoot, block, blockCreator)
	if err != nil {
		return nil, err
	}

	_, root := txn.Commit()

	return &BlockResult{
		Root: root,
	}, nil
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
	// Grab the parent block
	parentHash := childBlock.ParentHash()
	parent, ok := b.readHeader(parentHash)

	if !ok {
		b.logger.Error(fmt.Sprintf(
			"parent of %s (%d) not found: %s",
			childBlock.Hash().String(),
			childBlock.Number(),
			parentHash,
		))

		return ErrParentNotFound
	}

	// Make sure the hash is valid
	if parent.Hash == types.ZeroHash {
		return ErrInvalidParentHash
	}

	// Make sure the hashes match up
	if parentHash != parent.Hash {
		return ErrParentHashMismatch
	}

	// Make sure the block numbers are correct
	if childBlock.Number()-1 != parent.Number {
		b.logger.Error(fmt.Sprintf(
			"number sequence not correct at %d and %d",
			childBlock.Number(),
			parent.Number,
		))

		return ErrInvalidBlockSequence
	}

	return nil
}

// verifyBlockBody verifies that the block body is valid. This means checking:
// - The trie roots match up (state, transactions, receipts, uncles)
// - The receipts match up
// - The execution result matches up
func (b *Blockchain) verifyBlockBody(block *types.Block) error {
	// Execute the transactions in the block and grab the result
	blockResult, executeErr := b.executeBlockTransactions(block)
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

// readHeader Returns the header using the hash
func (b *Blockchain) readHeader(hash types.Hash) (*types.Header, bool) {
	// Try to find a hit in the headers cache
	h, ok := b.headersCache.Get(hash)
	if ok {
		// Hit, return the3 header
		header, ok := h.(*types.Header)
		if !ok {
			return nil, false
		}

		return header, true
	}

	// Cache miss, load it from the DB
	hh, err := b.db.ReadHeader(hash)
	if err != nil {
		return nil, false
	}

	// Compute the header hash and update the cache
	hh.ComputeHash()
	b.headersCache.Add(hash, hh)

	return hh, true
}

// GetHeaderByNumber returns the header using the block number
func (b *Blockchain) GetHeaderByNumber(n uint64) (*types.Header, bool) {
	hash, ok := b.db.ReadCanonicalHash(n)
	if !ok {
		return nil, false
	}

	h, ok := b.readHeader(hash)
	if !ok {
		return nil, false
	}

	return h, true
}

// GetBlockByHash returns the block using the block hash
func (b *Blockchain) GetBlockByHash(hash types.Hash, full bool) (*types.Block, bool) {
	header, ok := b.readHeader(hash)
	if !ok {
		return nil, false
	}

	block := &types.Block{
		Header: header,
	}

	if !full || header.Number == 0 {
		return block, true
	}

	return block, true
}

// GetBlockByNumber returns the block using the block number
func (b *Blockchain) GetBlockByNumber(blockNumber uint64, full bool) (*types.Block, bool) {
	blockHash, ok := b.db.ReadCanonicalHash(blockNumber)
	if !ok {
		return nil, false
	}

	// if blockNumber 0 (genesis block), do not try and get the full block
	if blockNumber == uint64(0) {
		full = false
	}

	return b.GetBlockByHash(blockHash, full)
}

// writeGenesis wrapper for the genesis write function
func (b *Blockchain) writeGenesis(genesis *chain.Genesis) error {
	header := genesis.GenesisHeader()
	header.ComputeHash()

	if err := b.writeGenesisImpl(header); err != nil {
		return err
	}

	return nil
}

// writeGenesisImpl writes the genesis file to the DB + blockchain reference
func (b *Blockchain) writeGenesisImpl(header *types.Header) error {
	// Update the reference
	b.genesis = header.Hash

	// Update the DB
	if err := b.db.WriteHeader(header); err != nil {
		return err
	}

	// Advance the head
	if _, err := b.advanceHead(header); err != nil {
		return err
	}

	// Create an event and send it to the stream
	event := &Event{}
	event.AddNewHeader(header)
	b.stream.push(event)

	return nil
}

// writeCanonicalHeader writes the new header
func (b *Blockchain) writeCanonicalHeader(event *Event, h *types.Header) error {
	if err := b.db.WriteCanonicalHeader(h, big.NewInt(0)); err != nil {
		return err
	}

	event.Type = EventHead
	event.AddNewHeader(h)
	event.SetDifficulty(big.NewInt(0))

	b.setCurrentHeader(h)

	return nil
}

// advanceHead Sets the passed in header as the new head of the chain
func (b *Blockchain) advanceHead(newHeader *types.Header) (*big.Int, error) {
	// Write the current head hash into storage
	if err := b.db.WriteHeadHash(newHeader.Hash); err != nil {
		return nil, err
	}

	// Write the current head number into storage
	if err := b.db.WriteHeadNumber(newHeader.Number); err != nil {
		return nil, err
	}

	// Matches the current head number with the current hash
	if err := b.db.WriteCanonicalHash(newHeader.Number, newHeader.Hash); err != nil {
		return nil, err
	}

	// Update the blockchain reference
	b.setCurrentHeader(newHeader)

	return big.NewInt(0), nil
}

// WriteBlock writes a single block to the local blockchain.
// It doesn't do any kind of verification, only commits the block to the DB
func (b *Blockchain) WriteBlock(block *types.Block, source string) error {
	b.writeLock.Lock()
	defer b.writeLock.Unlock()

	if block.Number() <= b.Header().Number {
		b.logger.Info("block already inserted", "block", block.Number(), "source", source)
		return nil
	}

	header := block.Header

	// Write the header to the chain
	evnt := &Event{Source: source}
	if err := b.writeHeaderImpl(evnt, header); err != nil {
		return err
	}

	// update snapshot
	if err := b.consensus.ProcessHeaders([]*types.Header{header}); err != nil {
		return err
	}

	b.dispatchEvent(evnt)

	logArgs := []any{
		"number", header.Number,
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

// writeHeaderImpl writes a block and the data, assumes the genesis is already set
func (b *Blockchain) writeHeaderImpl(evnt *Event, header *types.Header) error {
	currentHeader := b.Header()

	// Write the data
	if header.ParentHash == currentHeader.Hash {
		// Fast path to save the new canonical header
		return b.writeCanonicalHeader(evnt, header)
	}

	if err := b.db.WriteHeader(header); err != nil {
		return err
	}

	// Update the headers cache
	b.headersCache.Add(header.Hash, header)

	return nil
}

// Close closes the DB connection
func (b *Blockchain) Close() error {
	return b.db.Close()
}

func (b *Blockchain) GetRpcClient() bind.ContractBackend {
	return b.rpcClient.GetClient()
}

func (b *Blockchain) GetEthereumClient() *ethereum.Client {
	return b.rpcClient
}

func (b *Blockchain) GetMonitor() *monitor.Monitor {
	return b.monitor
}

func (b *Blockchain) GetNodeStorageAddress() types.Address {
	return b.nodeStorageAddress
}
