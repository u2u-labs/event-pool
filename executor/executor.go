package executor

import (
	"context"
	"fmt"
	"sync"

	"event-pool/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// ContractExecutor implements the p2p.Executor interface
type ContractExecutor struct {
	mutex  sync.RWMutex
	logger *zap.SugaredLogger
}

// NewContractExecutor creates a new executor for contract operations
func NewContractExecutor(logger *zap.SugaredLogger) *ContractExecutor {
	return &ContractExecutor{
		logger: logger,
	}
}

// ReconstructChain handles chain reconstruction logic
func (e *ContractExecutor) ReconstructChain(ctx context.Context, chainID int64) {
	e.logger.Infof("Reconstructing chain %d", chainID)
	// Your chain reconstruction logic here
}

// RegisterContract adds a contract to the registry
func (e *ContractExecutor) RegisterContract(ctx context.Context, chainID int64, contractAddress string, metadata map[string]string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return nil
}

// HasContract checks if a contract exists in the registry
func (e *ContractExecutor) HasContract(ctx context.Context, chainID int64, contractAddress string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return true
}

// GetContract retrieves a contract from the registry
func (e *ContractExecutor) GetContract(ctx context.Context, chainID int64, contractAddress string) (*p2p.ContractInfo, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return nil, false
}

// ValidateContract validates contract responses from peers and implements consensus logic
func (e *ContractExecutor) ValidateContract(ctx context.Context, chainID int64, contractAddress string,
	responses map[peer.ID]*p2p.ContractResponse) (bool, *p2p.ContractInfo, error) {

	// Count "found" responses
	foundCount := 0
	var foundInfo *p2p.ContractInfo

	totalResponses := len(responses)
	if totalResponses == 0 {
		return false, nil, fmt.Errorf("no responses received")
	}

	for _, resp := range responses {
		if resp.Found {
			foundCount++
			if foundInfo == nil {
				info := resp.Contract
				foundInfo = &info
			}
		}
	}

	if !passQuorum(totalResponses, foundCount) {
		return false, nil, fmt.Errorf("(only %d peers found the contract) total: %d",
			foundCount, totalResponses)
	}

	return true, foundInfo, nil
}

func passQuorum(n, q int) bool {
	if n < 1 {
		// No peers, so pass quorum
		return true
	}
	// Majority rule: pass quorum if 2/3+1 peers agree
	f := (n - 1) / 3
	return q >= 2*f+1
}

func (e *ContractExecutor) LoadAllChainData(ctx context.Context) error {
	// TODO: load data from db
	return nil
}
