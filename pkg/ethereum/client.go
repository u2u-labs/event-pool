package ethereum

import (
	"context"
	"event-pool/prisma/db"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"log"

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
}

func NewClient(rpcURL string, chainID, blockTime int, db *db.PrismaClient) (*Client, error) {
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

	log.Printf("Filtering logs with query: FromBlock=%s, ToBlock=%s, Contract=%s, EventSig=%s",
		fromBlock.String(), toBlock.String(), contractAddress.Hex(), eventSignature.Hex())

	logs, err := c.client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("Error filtering logs: %v", err)
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	log.Printf("Found %d logs for contract %s", len(logs), contractAddress.Hex())
	for i, eventLog := range logs {
		log.Printf("Log %d: Block=%d, TxHash=%s, Index=%d, Topics=%v, Data=%s",
			i, eventLog.BlockNumber, eventLog.TxHash.Hex(), eventLog.Index, eventLog.Topics, common.Bytes2Hex(eventLog.Data))

		// Get the contract from the database
		contract, err := c.db.Contract.FindUnique(
			db.Contract.ChainIDAddressEventSignature(
				db.Contract.ChainID.Equals(chainID),
				db.Contract.Address.Equals(strings.ToLower(contractAddress.Hex())),
				db.Contract.EventSignature.Equals(eventSignature.Hex()),
			),
		).Exec(ctx)

		if err != nil {
			log.Printf("Error finding contract: %v", err)
			continue
		}

		// Check if this event log already exists to avoid duplicates
		existingLog, err := c.db.EventLog.FindFirst(
			db.EventLog.ContractID.Equals(contract.ID),
			db.EventLog.BlockNumber.Equals(int(eventLog.BlockNumber)),
			db.EventLog.TxHash.Equals(strings.ToLower(eventLog.TxHash.Hex())),
			db.EventLog.LogIndex.Equals(int(eventLog.Index)),
		).Exec(ctx)

		if err == nil && existingLog != nil {
			log.Printf("Event log already exists, skipping: Block=%d, TxHash=%s, Index=%d",
				eventLog.BlockNumber, eventLog.TxHash.Hex(), eventLog.Index)
			continue
		}

		// Decode the event data into human-readable JSON
		decodedData, err := c.decoder.DecodeEvent(eventSignature.Hex(), eventLog.Data, eventLog.Topics)
		if err != nil {
			log.Printf("Error decoding event data: %v", err)
			// Fall back to hex data if decoding fails
			decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(eventLog.Data))
		}

		log.Printf("Decoded event data: %s", decodedData)

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
			log.Printf("Error creating event log: %v", err)
		}
	}

	return logs, nil
}

func (c *Client) SubscribeToLogs(ctx context.Context, contractAddress common.Address, eventSignature common.Hash) (<-chan Log, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	log.Printf("Subscribing to logs for contract %s", contractAddress.Hex())

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
				log.Printf("Subscription error for contract %s: %v", contractAddress.Hex(), err)
			case <-ctx.Done():
				log.Printf("Unsubscribing from logs for contract %s", contractAddress.Hex())
				sub.Unsubscribe()
				close(logs)
				return
			}
		}
	}()

	log.Printf("Successfully subscribed to logs for contract %s", contractAddress.Hex())
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
