package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"event-pool/pkg/eventproducer"
	"event-pool/prisma/db"
	types2 "event-pool/types"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Client struct {
	client    *ethclient.Client
	chainID   int
	blockTime int
	mu        sync.RWMutex
	db        *db.PrismaClient
	decoder   *EventDecoder
	logger    *zap.SugaredLogger
	producer  eventproducer.EventProducer
}

func NewClient(rpcURL string, chainID, blockTime int, db *db.PrismaClient, logger *zap.SugaredLogger, producer eventproducer.EventProducer) (*Client, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	return &Client{
		client:    client,
		chainID:   chainID,
		blockTime: blockTime,
		db:        db,
		decoder:   NewEventDecoder(),
		logger:    logger,
		producer:  producer,
	}, nil
}

func (c *Client) Close() {
	c.client.Close()
}

func (c *Client) GetLatestBlock() (uint64, error) {
	header, err := c.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}
	return header.Number.Uint64(), nil
}

func (c *Client) FilterLogs(ctx context.Context, contractAddress common.Address, eventSignature common.Hash, fromBlock, toBlock *big.Int, chainID int) ([]Log, error) {
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	c.logger.Infof("Filtering logs with query: FromBlock=%s, ToBlock=%s, Contract=%s, EventSig=%s",
		fromBlock.String(), toBlock.String(), contractAddress.Hex(), eventSignature.Hex())

	logs, err := c.client.FilterLogs(ctx, query)
	if err != nil {
		c.logger.Infof("Error filtering logs: %v", err)
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	c.logger.Infof("Found %d logs for contract %s", len(logs), contractAddress.Hex())

	timestampCache := make(map[uint64]uint64)
	for i, eventLog := range logs {
		c.logger.Infof("Log %d: Block=%d, TxHash=%s, Index=%d, Topics=%v, Data=%s",
			i, eventLog.BlockNumber, eventLog.TxHash.Hex(), eventLog.Index, eventLog.Topics, common.Bytes2Hex(eventLog.Data))

		if err := c.processEventLog(ctx, eventLog, contractAddress, eventSignature, chainID, timestampCache); err != nil {
			c.logger.Infof("Error processing event log %d: %v", i, err)
			continue
		}
	}

	return logs, nil
}

func (c *Client) processEventLog(ctx context.Context, eventLog types.Log, contractAddress common.Address, eventSignature common.Hash, chainID int, timestampCache map[uint64]uint64) error {
	contract, err := c.db.Contract.FindUnique(
		db.Contract.ChainIDAddressEventSignature(
			db.Contract.ChainID.Equals(chainID),
			db.Contract.Address.Equals(strings.ToLower(contractAddress.Hex())),
			db.Contract.EventSignature.Equals(eventSignature.Hex()),
		),
	).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error finding contract: %w", err)
	}

	// Check if this event log already exists to avoid duplicates
	existingLog, err := c.db.EventLog.FindFirst(
		db.EventLog.ContractID.Equals(contract.ID),
		db.EventLog.BlockNumber.Equals(int(eventLog.BlockNumber)),
		db.EventLog.TxHash.Equals(strings.ToLower(eventLog.TxHash.Hex())),
		db.EventLog.LogIndex.Equals(int(eventLog.Index)),
	).Exec(ctx)

	if err == nil && existingLog != nil {
		c.logger.Infof("Event log already exists, skipping: Block=%d, TxHash=%s, Index=%d",
			eventLog.BlockNumber, eventLog.TxHash.Hex(), eventLog.Index)
		return nil
	}

	// Decode the event data into human-readable JSON
	decodedData, err := c.decoder.DecodeEvent(eventSignature.Hex(), eventLog.Data, eventLog.Topics)
	if err != nil {
		c.logger.Infof("Error decoding event data: %v", err)
		// Fall back to hex data if decoding fails
		decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(eventLog.Data))
	}

	c.logger.Infof("Decoded event data: %s", decodedData)

	params, err := c.decoder.DecodeEventToMap(eventSignature.Hex(), eventLog.Data, eventLog.Topics)
	if err != nil {
		c.logger.Errorf("Error decoding event: %v", err)
	} else {
		ts, ok := timestampCache[eventLog.BlockNumber]
		if !ok {
			block, err := c.client.BlockByNumber(ctx, big.NewInt(int64(eventLog.BlockNumber)))
			if err != nil {
				return fmt.Errorf("failed to get block: %w", err)
			}
			ts = block.Time()
			timestampCache[eventLog.BlockNumber] = ts
		}

		pl := types2.EventRunnerPayload{
			Params:          params,
			TransactionHash: eventLog.TxHash.Hex(),
			Timestamp:       ts,
			BlockNumber:     eventLog.BlockNumber,
			BlockHash:       eventLog.BlockHash.Hex(),
			ContractAddress: contractAddress.Hex(),
			EventName:       eventLog.Topics[0].Hex(),
			EventSignature:  eventSignature.Hex(),
			EventData:       eventLog.Data,
			EventLogIndex:   eventLog.Index,
			ChainID:         chainID,
		}
		c.producer.Publish(pl)
	}

	_, err = c.db.EventLog.CreateOne(
		db.EventLog.Contract.Link(
			db.Contract.ID.Equals(contract.ID),
		),
		db.EventLog.BlockNumber.Set(int(eventLog.BlockNumber)),
		db.EventLog.TxHash.Set(strings.ToLower(eventLog.TxHash.Hex())),
		db.EventLog.LogIndex.Set(int(eventLog.Index)),
		db.EventLog.Data.Set(decodedData),
	).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error creating event log: %w", err)
	}

	return nil
}

func (c *Client) SubscribeToLogs(ctx context.Context, contractAddress common.Address, eventSignature common.Hash) (<-chan Log, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	c.logger.Infof("Subscribing to logs for contract %s", contractAddress.Hex())

	logs := make(chan Log)
	sub, err := c.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to logs: %w", err)
	}

	// Handle subscription errors
	go func() {
		for {
			select {
			case err := <-sub.Err():
				c.logger.Infof("Subscription error for contract %s: %v", contractAddress.Hex(), err)
			case <-ctx.Done():
				c.logger.Infof("Unsubscribing from logs for contract %s", contractAddress.Hex())
				sub.Unsubscribe()
				close(logs)
				return
			}
		}
	}()

	c.logger.Infof("Successfully subscribed to logs for contract %s", contractAddress.Hex())
	return logs, nil
}

// RegisterEventABI registers an event ABI with the decoder
func (c *Client) RegisterEventABI(eventSignature string, eventABI string) error {
	return c.decoder.RegisterEvent(eventSignature, eventABI)
}

// GetDecoder returns the event decoder
func (c *Client) GetDecoder() *EventDecoder {
	return c.decoder
}

func (c *Client) GetClient() *ethclient.Client {
	return c.client
}

func (c *Client) GetChainId() int {
	return c.chainID
}
