package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"event-pool/internal/listener"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"

	"github.com/ethereum/go-ethereum/common"
	"github.com/hibiken/asynq"
)

const (
	TypeBackfill = "backfill"
)

type BackfillPayload struct {
	ChainID      int    `json:"chain_id"`
	ContractAddr string `json:"contract_addr"`
	EventSig     string `json:"event_sig"`
	StartBlock   int64  `json:"start_block"`
}

type Worker struct {
	server     *asynq.Server
	ethClients map[int]*ethereum.Client
	redisAddr  string
	db         *db.PrismaClient
	decoder    *ethereum.EventDecoder
}

func NewWorker(redisAddr string, ethClients map[int]*ethereum.Client, db *db.PrismaClient) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{Concurrency: 10},
	)

	return &Worker{
		server:     srv,
		ethClients: ethClients,
		redisAddr:  redisAddr,
		db:         db,
		decoder:    ethereum.NewEventDecoder(),
	}
}

func (w *Worker) Start() error {
	log.Printf("Starting worker with Redis address: %s", w.redisAddr)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeBackfill, w.handleBackfill)

	log.Printf("Worker registered handler for task type: %s", TypeBackfill)

	return w.server.Run(mux)
}

func (w *Worker) handleBackfill(ctx context.Context, t *asynq.Task) error {
	log.Printf("Starting to process backfill task: %s", t.Type())

	var p BackfillPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		log.Printf("Error unmarshaling payload: %v", err)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("Backfill task payload: ChainID=%d, ContractAddr=%s, EventSig=%s, StartBlock=%d",
		p.ChainID, p.ContractAddr, p.EventSig, p.StartBlock)

	// Get the appropriate Ethereum client for the chain
	client, ok := w.ethClients[p.ChainID]
	if !ok {
		log.Printf("No Ethereum client found for chain ID %d", p.ChainID)
		return fmt.Errorf("no Ethereum client found for chain ID %d", p.ChainID)
	}

	log.Printf("Found Ethereum client for chain ID %d", p.ChainID)

	// Get the contract from the database
	contract, err := w.db.Contract.FindUnique(
		db.Contract.ChainIDAddressEventSignature(
			db.Contract.ChainID.Equals(p.ChainID),
			db.Contract.Address.Equals(strings.ToLower(p.ContractAddr)),
			db.Contract.EventSignature.Equals(p.EventSig),
		),
	).Exec(ctx)

	if err != nil {
		log.Printf("Error finding contract in database: %v", err)
		return fmt.Errorf("failed to find contract: %w", err)
	}

	log.Printf("Found contract in database with ID: %s", contract.ID)

	// Create event listener for backfilling
	listener := listener.NewEventListener(
		client,
		p.ChainID,
		ethereum.HexToAddress(p.ContractAddr),
		ethereum.HexToHash(p.EventSig),
		big.NewInt(p.StartBlock),
	)

	log.Printf("Created event listener for contract %s", p.ContractAddr)

	// Start backfilling
	eventChan := make(chan ethereum.Log, 100)
	go func() {
		log.Printf("Starting backfill for contract %s", p.ContractAddr)
		if err := listener.Start(ctx, eventChan); err != nil {
			log.Printf("Error during backfill: %v", err)
		}
		log.Printf("Backfill completed for contract %s", p.ContractAddr)
	}()

	// Process events
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context done, stopping backfill for contract %s", p.ContractAddr)
			return ctx.Err()
		case eventLog, ok := <-eventChan:
			if !ok {
				log.Printf("Event channel closed for contract %s", p.ContractAddr)
				return nil
			}
			log.Printf("Processing event from contract %s: Block=%d, TxHash=%s, Index=%d",
				p.ContractAddr, eventLog.BlockNumber, eventLog.TxHash.Hex(), eventLog.Index)

			// Decode the event data into human-readable JSON
			// Use the event signature from the log's first topic
			eventSig := eventLog.Topics[0].Hex()
			decodedData, err := w.decoder.DecodeEvent(eventSig, eventLog.Data, eventLog.Topics)
			if err != nil {
				log.Printf("Error decoding event data: %v", err)
				// Fall back to hex data if decoding fails
				decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(eventLog.Data))
			}

			log.Printf("Decoded event data: %s", decodedData)

			// Check if this event log already exists to avoid duplicates
			existingLog, err := w.db.EventLog.FindFirst(
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

			// Create event log with decoded data
			_, err = w.db.EventLog.CreateOne(
				db.EventLog.Contract.Link(
					db.Contract.ID.Equals(strings.ToLower(contract.ID)),
				),
				db.EventLog.BlockNumber.Set(int(eventLog.BlockNumber)),
				db.EventLog.TxHash.Set(strings.ToLower(eventLog.TxHash.Hex())),
				db.EventLog.LogIndex.Set(int(eventLog.Index)),
				db.EventLog.Data.Set(decodedData),
			).Exec(ctx)

			if err != nil {
				log.Printf("Error storing event in database: %v", err)
				continue
			}

			log.Printf("Stored event in database for contract %s", p.ContractAddr)
		}
	}
}

func (w *Worker) Shutdown() error {
	log.Printf("Shutting down worker...")
	w.server.Shutdown()
	return nil
}

// EnqueueTask enqueues a new task to be processed by the worker
func (w *Worker) EnqueueTask(taskType string, payload []byte) error {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: w.redisAddr})
	defer client.Close()

	task := asynq.NewTask(taskType, payload)
	_, err := client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("Successfully enqueued task of type %s", taskType)
	return nil
}
