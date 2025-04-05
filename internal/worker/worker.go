package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"event-pool/internal/listener"
	"event-pool/internal/monitor"
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
	monitor    *monitor.Monitor
}

func NewWorker(redisAddr string, ethClients map[int]*ethereum.Client, db *db.PrismaClient, monitor *monitor.Monitor) *Worker {
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
		monitor:    monitor,
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

	client, ok := w.ethClients[p.ChainID]
	if !ok {
		log.Printf("No Ethereum client found for chain ID %d", p.ChainID)
		return fmt.Errorf("no Ethereum client found for chain ID %d", p.ChainID)
	}

	log.Printf("Found Ethereum client for chain ID %d", p.ChainID)

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

	// Check if the contract is already being backfilled
	if w.monitor != nil && w.monitor.IsContractBackfilling(contract.ID) {
		log.Printf("Contract %s is already being backfilled, skipping", contract.ID)
		return nil
	}

	// Register the contract with the monitor if it's not already being monitored
	if w.monitor != nil && !w.monitor.IsContractReadyForMonitoring(contract.ID) {
		log.Printf("Registering contract %s with monitor", contract.ID)
		if err := w.monitor.RegisterContract(ctx, contract); err != nil {
			log.Printf("Error registering contract with monitor: %v", err)
			// Continue with backfill even if registration fails
		}
	}

	// Mark the contract as being backfilled
	if w.monitor != nil {
		w.monitor.MarkContractBackfilling(contract.ID)
		log.Printf("Marked contract %s as being backfilled", contract.ID)
	}

	// Create a done channel to signal when backfill is complete
	done := make(chan struct{})

	// Create event listener for backfilling
	listener := listener.NewEventListener(
		client,
		p.ChainID,
		ethereum.HexToAddress(p.ContractAddr),
		ethereum.HexToHash(p.EventSig),
		big.NewInt(p.StartBlock),
	)

	log.Printf("Created event listener for contract %s", p.ContractAddr)

	eventChan := make(chan ethereum.Log, 100)
	go func() {
		log.Printf("Starting backfill for contract %s", p.ContractAddr)
		if err := listener.Start(ctx, eventChan); err != nil {
			log.Printf("Error during backfill: %v", err)
		}
		log.Printf("Backfill completed for contract %s", p.ContractAddr)

		// Signal that backfill is complete
		close(done)

		// Mark the contract as having completed backfill
		if w.monitor != nil {
			w.monitor.MarkContractBackfillComplete(contract.ID)
			log.Printf("Marked contract %s as having completed backfill", contract.ID)
		}
	}()

	// Process events from the channel
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping backfill for contract %s", p.ContractAddr)

			if w.monitor != nil {
				w.monitor.MarkContractBackfillComplete(contract.ID)
				log.Printf("Marked contract %s as having completed backfill (context cancelled)", contract.ID)
			}
			return nil
		case <-done:
			log.Printf("Backfill process completed for contract %s", p.ContractAddr)
			return nil
		case event, ok := <-eventChan:
			if !ok {
				log.Printf("Event channel closed for contract %s", p.ContractAddr)
				return nil
			}

			decodedData, err := client.GetDecoder().DecodeEvent(p.EventSig, event.Data, event.Topics)
			if err != nil {
				log.Printf("Error decoding event data: %v", err)
				decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(event.Data))
			}

			existingEvent, err := w.db.EventLog.FindFirst(
				db.EventLog.BlockNumber.Equals(int(event.BlockNumber)),
				db.EventLog.TxHash.Equals(strings.ToLower(event.TxHash.Hex())),
				db.EventLog.LogIndex.Equals(int(event.Index)),
			).Exec(ctx)

			if err == nil && existingEvent != nil {
				log.Printf("Event log already exists, skipping: Block=%d, TxHash=%s, Index=%d",
					event.BlockNumber, event.TxHash.Hex(), event.Index)
				continue
			}

			_, err = w.db.EventLog.CreateOne(
				db.EventLog.Contract.Link(
					db.Contract.ID.Equals(contract.ID),
				),
				db.EventLog.BlockNumber.Set(int(event.BlockNumber)),
				db.EventLog.TxHash.Set(strings.ToLower(event.TxHash.Hex())),
				db.EventLog.LogIndex.Set(int(event.Index)),
				db.EventLog.Data.Set(decodedData),
			).Exec(ctx)

			if err != nil {
				log.Printf("Error storing event: %v", err)
				continue
			}

			log.Printf("Stored new event: Block=%d, TxHash=%s",
				event.BlockNumber, event.TxHash.Hex())
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

// IsContractReadyForMonitoring returns whether a contract is ready for monitoring
func (w *Worker) IsContractReadyForMonitoring(contractID string) bool {
	if w.monitor == nil {
		return false
	}
	return w.monitor.IsContractReadyForMonitoring(contractID)
}
