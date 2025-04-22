package state

import (
	"context"
	"encoding/json"
	"math"
	"math/big"

	"event-pool/chain"
	"event-pool/crypto"
	"event-pool/internal/monitor"
	"event-pool/pkg/ethereum"
	"event-pool/types"
	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
)

type Executor struct {
	logger   *zap.SugaredLogger
	config   *chain.Params
	state    State
	txnCache *lru.Cache // used for re-using pre-transformed txn outside the building block scope
	ssCache  *lru.Cache // transition snapshot id cache. Necessary in order to handle block already inserted
	chainId  uint64

	FnGetRpcClient func() *ethereum.Client
	FnGetMonitor   func() *monitor.Monitor
}

// NewExecutor creates a new executor
func NewExecutor(config *chain.Params, s State, logger *zap.SugaredLogger) *Executor {
	txnCache, err := lru.New(10)
	if err != nil {
		logger.Error("failed to init cache", "err", err)
	}
	ssCache, err := lru.New(10)
	if err != nil {
		logger.Error("failed to init cache", "err", err)
	}

	return &Executor{
		logger:   logger,
		config:   config,
		state:    s,
		txnCache: txnCache,
		ssCache:  ssCache,
	}
}

type BlockResult struct {
	Root types.Hash
}

func (e *Executor) BeginTxn(
	parentRoot types.Hash,
	header *types.Header,
	coinbaseReceiver types.Address,
) (*Transition, error) {
	config := e.config.Forks.At(header.Number)

	auxSnap2, err := e.state.NewSnapshotAt(parentRoot)
	if err != nil {
		return nil, err
	}

	newTxn := NewTxn(e.state, auxSnap2)

	txn := &Transition{
		logger:           e.logger,
		r:                e,
		ctx:              context.Background(),
		state:            newTxn,
		auxState:         e.state,
		config:           config,
		gasPool:          math.MaxUint64,
		FnGetEtherClient: e.FnGetRpcClient,
		chainId:          e.chainId,
		FnGetMonitor:     e.FnGetMonitor,
	}

	return txn, nil
}

// ProcessBlock already does all the handling of the whole process
func (e *Executor) ProcessBlock(
	parentRoot types.Hash,
	block *types.Block,
	blockCreator types.Address,
) (*Transition, error) {
	txn, err := e.BeginTxn(parentRoot, block.Header, blockCreator)
	if err != nil {
		return nil, err
	}

	for _, t := range block.Transactions {
		if err := txn.Write(t); err != nil {
			return nil, err
		}
	}

	return txn, nil
}

func (e *Executor) SetChainId(chainId uint64) {
	e.chainId = chainId
}

// -----------------------------------------------------------------------------

type Transition struct {
	logger *zap.SugaredLogger

	// dummy
	auxState State

	r       *Executor
	config  chain.ForksInTime
	state   *Txn
	ctx     context.Context
	gasPool uint64

	FnGetEtherClient func() *ethereum.Client
	chainId          uint64
	FnGetMonitor     func() *monitor.Monitor

	// result
}

func NewTransition(config chain.ForksInTime, radix *Txn) *Transition {
	return &Transition{
		config: config,
		state:  radix,
		r:      &Executor{},
	}
}

type TransitionApplicationError struct {
	Err           error
	IsRecoverable bool // Should the transaction be discarded, or put back in the queue.
}

func (e *TransitionApplicationError) Error() string {
	return e.Err.Error()
}

func NewTransitionApplicationError(err error, isRecoverable bool) *TransitionApplicationError {
	return &TransitionApplicationError{
		Err:           err,
		IsRecoverable: isRecoverable,
	}
}

var emptyFrom = types.Address{}

// Write writes another transaction to the executor
func (t *Transition) Write(txn *types.Transaction) error {
	signer := crypto.NewSigner(t.config.EIP155, uint64(t.r.config.ChainID))

	var err error
	if txn.From == emptyFrom {
		// Decrypt the from address
		txn.From, err = signer.Sender(txn)
		if err != nil {
			return NewTransitionApplicationError(err, false)
		}
	}

	// Make a local copy and apply the transaction
	msg := txn.Copy()
	_, e := t.Apply(msg)
	if e != nil {
		t.logger.Error("failed to apply tx", "err", e)

		return e
	}

	if t.config.Byzantium {
		// The suicided accounts are set as deleted for the next iteration
		t.state.CleanDeleteObjects(true)

	} else {
		objs := t.state.Commit(t.config.EIP155)
		ss, _ := t.state.snapshot.Commit(objs)
		t.state = NewTxn(t.auxState, ss)
	}

	return nil
}

// Apply applies a new transaction
func (t *Transition) Apply(msg *types.Transaction) (any, error) {
	s := t.state.Snapshot()
	result, err := t.apply(msg)

	if err != nil {
		t.state.RevertToSnapshot(s)
		return result, err
	}

	return result, err
}

// filter logs params
type FilterLogsParams struct {
	FromBlock       *big.Int
	ToBlock         *big.Int
	contractAddress common.Address
	eventSignature  common.Hash
}

func (t *Transition) apply(msg *types.Transaction) (any, error) {
	//txn := t.state

	params := FilterLogsParams{}
	if err := json.Unmarshal(msg.Input, &params); err != nil {
		t.logger.Errorw("failed to unmarshal logs params", "err", err)
		return nil, err
	}
	client := t.FnGetEtherClient()

	// Get logs for the block range
	logs, err := client.FilterLogs(
		t.ctx,
		params.contractAddress,
		params.eventSignature,
		params.FromBlock,
		params.ToBlock,
		int(t.chainId),
	)
	if err != nil {
		t.logger.Errorw("failed to get logs", "err", err)
		return nil, err
	}

	t.FnGetMonitor().ProcessLogsEvent(t.ctx, logs, client, params.eventSignature.String(), int64(t.chainId), params.contractAddress.String())
	// just increase the nonce to change state
	t.state.IncrNonce(msg.From)

	return true, nil
}

// Commit commits the final result
func (t *Transition) Commit() (Snapshot, types.Hash) {
	objs := t.state.Commit(t.config.EIP155)
	s2, root := t.state.snapshot.Commit(objs)

	return s2, types.BytesToHash(root)
}

func (t *Transition) WriteFailedReceipt(txn *types.Transaction) error {
	return nil
}
