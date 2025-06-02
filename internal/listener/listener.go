package listener

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"event-pool/pkg/ethereum"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum/common"
)

type EventListener struct {
	client        *ethereum.Client
	chainID       int
	contract      common.Address
	eventSig      common.Hash
	startBlock    *big.Int
	mu            sync.RWMutex
	rdb           *redis.Client
	logger        *zap.SugaredLogger
	backfillCache *sync.Map
}

func NewEventListener(client *ethereum.Client, chainID int, contract common.Address, eventSig common.Hash, startBlock *big.Int,
	rdb *redis.Client, metrics *sync.Map, logger *zap.SugaredLogger) *EventListener {
	return &EventListener{
		client:        client,
		chainID:       chainID,
		contract:      contract,
		eventSig:      eventSig,
		startBlock:    startBlock,
		mu:            sync.RWMutex{},
		rdb:           rdb,
		backfillCache: metrics,
		logger:        logger,
	}
}

func (l *EventListener) Start(ctx context.Context, eventChan chan<- ethereum.Log) error {
	l.logger.Infof("Starting event listener for contract %s on chain %d", l.contract.Hex(), l.chainID)

	if err := l.backfill(ctx, eventChan); err != nil {
		return fmt.Errorf("failed to backfill events: %w", err)
	}

	close(eventChan)
	return nil
}

func (l *EventListener) backfill(ctx context.Context, eventChan chan<- ethereum.Log) error {
	latestBlock, err := l.client.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	l.logger.Infof("Starting backfill from block %s to %d for contract %s",
		l.startBlock.String(), latestBlock, l.contract.Hex())

	// Process events in chunks to avoid timeout
	chunkSize := big.NewInt(2000)
	currentBlock := new(big.Int).Set(l.startBlock)
	endBlock := big.NewInt(int64(latestBlock))

	l.updateBackfillStatus(l.chainID, l.contract.Hex(), l.eventSig.Hex(), l.startBlock)

	for currentBlock.Cmp(endBlock) < 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			toBlock := new(big.Int).Add(currentBlock, chunkSize)
			if toBlock.Cmp(endBlock) > 0 {
				toBlock = endBlock
			}

			l.logger.Infof("Processing blocks %s to %s for contract %s",
				currentBlock.String(), toBlock.String(), l.contract.Hex())

			logs, err := l.client.FilterLogs(ctx, l.contract, l.eventSig, currentBlock, toBlock, l.chainID)
			if err != nil {
				return fmt.Errorf("failed to filter logs: %w", err)
			}

			for _, eventLog := range logs {
				select {
				case eventChan <- eventLog:
					l.logger.Infof("Sent event from block %d to channel", eventLog.BlockNumber)
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			currentBlock = new(big.Int).Add(toBlock, big.NewInt(1))

			l.updateBackfillStatus(l.chainID, l.contract.Hex(), l.eventSig.Hex(), currentBlock)
		}
	}

	l.clearBackfillStatus(l.chainID, l.contract.Hex(), l.eventSig.Hex())
	l.logger.Infof("Completed backfill for contract %s", l.contract.Hex())
	return nil
}

func (l *EventListener) updateBackfillStatus(chainId int, contractAddress string, eventSig string, currentBlock *big.Int) {
	key := strings.ToLower(fmt.Sprintf("backfill_status:%d/%s/%s", chainId, contractAddress, eventSig))
	l.backfillCache.Store(key, new(big.Int).Set(currentBlock))
}

func (l *EventListener) clearBackfillStatus(chainId int, contractAddress string, eventSig string) {
	key := strings.ToLower(fmt.Sprintf("backfill_status:%d/%s/%s", chainId, contractAddress, eventSig))
	l.backfillCache.Delete(key)
}

// func (l *EventListener) listen(ctx context.Context, eventChan chan<- ethereum.Log) error {
// 	l.logger.Infof()("Starting to listen for new events for contract %s", l.contract.Hex())

// 	// Get the latest block number
// 	latestBlock, err := l.client.GetLatestBlock()
// 	if err != nil {
// 		return fmt.Errorf("failed to get latest block: %w", err)
// 	}

// 	l.logger.Infof()("Starting to poll for new events from block %d", latestBlock)

// 	// Use polling instead of WebSocket subscriptions
// 	pollTicker := time.NewTicker(5 * time.Second)
// 	defer pollTicker.Stop()

// 	lastProcessedBlock := latestBlock

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		case <-pollTicker.C:
// 			// Get the latest block
// 			currentBlock, err := l.client.GetLatestBlock()
// 			if err != nil {
// 				l.logger.Infof()("ERROR: Failed to get latest block: %v", err)
// 				continue
// 			}

// 			// Check if we have new blocks to process
// 			if currentBlock > lastProcessedBlock {
// 				l.logger.Infof()("Processing new blocks %d to %d for contract %s",
// 					lastProcessedBlock+1, currentBlock, l.contract.Hex())

// 				// Process blocks in batches to avoid timeouts
// 				batchSize := uint64(10)
// 				for fromBlock := lastProcessedBlock + 1; fromBlock <= currentBlock; fromBlock += batchSize {
// 					toBlock := fromBlock + batchSize - 1
// 					if toBlock > currentBlock {
// 						toBlock = currentBlock
// 					}

// 					l.logger.Infof()("Fetching logs for blocks %d to %d", fromBlock, toBlock)

// 					// Get logs for the block range
// 					logs, err := l.client.FilterLogs(
// 						ctx,
// 						l.contract,
// 						l.eventSig,
// 						big.NewInt(int64(fromBlock)),
// 						big.NewInt(int64(toBlock)),
// 						l.chainID,
// 					)

// 					if err != nil {
// 						l.logger.Infof()("ERROR: Failed to fetch logs: %v", err)
// 						continue
// 					}

// 					l.logger.Infof()("Found %d logs for blocks %d to %d", len(logs), fromBlock, toBlock)

// 					// Process each log
// 					for _, eventLog := range logs {
// 						select {
// 						case eventChan <- eventLog:
// 							l.logger.Infof()("Sent new event from block %d to channel", eventLog.BlockNumber)
// 						case <-ctx.Done():
// 							return ctx.Err()
// 						}
// 					}

// 					// Update the last processed block
// 					lastProcessedBlock = toBlock
// 				}
// 			} else {
// 				l.logger.Infof()("No new blocks to process. Current: %d, Last: %d", currentBlock, lastProcessedBlock)
// 			}
// 		}
// 	}
// }
