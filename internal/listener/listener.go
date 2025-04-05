package listener

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"

	"event-pool/pkg/ethereum"

	"github.com/ethereum/go-ethereum/common"
)

type EventListener struct {
	client     *ethereum.Client
	chainID    int
	contract   common.Address
	eventSig   common.Hash
	startBlock *big.Int
	mu         sync.RWMutex
}

func NewEventListener(client *ethereum.Client, chainID int, contract common.Address, eventSig common.Hash, startBlock *big.Int) *EventListener {
	return &EventListener{
		client:     client,
		chainID:    chainID,
		contract:   contract,
		eventSig:   eventSig,
		startBlock: startBlock,
	}
}

func (l *EventListener) Start(ctx context.Context, eventChan chan<- ethereum.Log) error {
	log.Printf("Starting event listener for contract %s on chain %d", l.contract.Hex(), l.chainID)

	// First, backfill historical events
	if err := l.backfill(ctx, eventChan); err != nil {
		return fmt.Errorf("failed to backfill events: %w", err)
	}

	// Then start listening for new events
	return l.listen(ctx, eventChan)
}

func (l *EventListener) backfill(ctx context.Context, eventChan chan<- ethereum.Log) error {
	latestBlock, err := l.client.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	log.Printf("Starting backfill from block %s to %d for contract %s",
		l.startBlock.String(), latestBlock, l.contract.Hex())

	// Process events in chunks to avoid timeout
	chunkSize := big.NewInt(2000)
	currentBlock := new(big.Int).Set(l.startBlock)
	endBlock := big.NewInt(int64(latestBlock))

	for currentBlock.Cmp(endBlock) < 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			toBlock := new(big.Int).Add(currentBlock, chunkSize)
			if toBlock.Cmp(endBlock) > 0 {
				toBlock = endBlock
			}

			log.Printf("Processing blocks %s to %s for contract %s",
				currentBlock.String(), toBlock.String(), l.contract.Hex())

			logs, err := l.client.FilterLogs(ctx, l.contract, l.eventSig, currentBlock, toBlock, l.chainID)

			if err != nil {
				return fmt.Errorf("failed to filter logs: %w", err)
			}

			for _, eventLog := range logs {
				select {
				case eventChan <- eventLog:
					log.Printf("Sent event from block %d to channel", eventLog.BlockNumber)
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			currentBlock = new(big.Int).Add(toBlock, big.NewInt(1))
		}
	}

	log.Printf("Completed backfill for contract %s", l.contract.Hex())
	return nil
}

func (l *EventListener) listen(ctx context.Context, eventChan chan<- ethereum.Log) error {
	log.Printf("Starting to listen for new events for contract %s", l.contract.Hex())

	logs, err := l.client.SubscribeToLogs(ctx, l.contract, l.eventSig)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case eventLog, ok := <-logs:
			if !ok {
				return fmt.Errorf("log channel closed")
			}
			select {
			case eventChan <- eventLog:
				log.Printf("Sent new event from block %d to channel", eventLog.BlockNumber)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
