package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"event-pool/internal/listener"
	"event-pool/internal/monitor"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"
	"go.uber.org/zap"

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
	logger     *zap.SugaredLogger
}

func NewWorker(redisAddr string, ethClients map[int]*ethereum.Client, db *db.PrismaClient, monitor *monitor.Monitor, logger *zap.SugaredLogger) *Worker {
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
		logger:     logger,
	}
}

func (w *Worker) Start() error {
	w.logger.Infof("Starting worker with Redis address: %s", w.redisAddr)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeBackfill, w.handleBackfill)

	w.logger.Infof("Worker registered handler for task type: %s", TypeBackfill)

	return w.server.Run(mux)
}

func (w *Worker) handleBackfill(ctx context.Context, t *asynq.Task) error {
	w.logger.Infof("Starting to process backfill task: %s", t.Type())

	var p BackfillPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		w.logger.Infof("Error unmarshaling payload: %v", err)
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	w.logger.Infof("Backfill task payload: ChainID=%d, ContractAddr=%s, EventSig=%s, StartBlock=%d",
		p.ChainID, p.ContractAddr, p.EventSig, p.StartBlock)

	client, ok := w.ethClients[p.ChainID]
	if !ok {
		w.logger.Infof("No Ethereum client found for chain ID %d", p.ChainID)
		return fmt.Errorf("no Ethereum client found for chain ID %d", p.ChainID)
	}

	w.logger.Infof("Found Ethereum client for chain ID %d", p.ChainID)

	contract, err := w.db.Contract.FindUnique(
		db.Contract.ChainIDAddressEventSignature(
			db.Contract.ChainID.Equals(p.ChainID),
			db.Contract.Address.Equals(strings.ToLower(p.ContractAddr)),
			db.Contract.EventSignature.Equals(p.EventSig),
		),
	).Exec(ctx)

	if err != nil {
		w.logger.Infof("Error finding contract in database: %v", err)
		return fmt.Errorf("failed to find contract: %w", err)
	}

	w.logger.Infof("Found contract in database with ID: %s", contract.ID)

	// Check if the contract is already being backfilled
	if w.monitor != nil && w.monitor.IsContractBackfilling(contract.ID) {
		w.logger.Infof("Contract %s is already being backfilled, skipping", contract.ID)
		return nil
	}

	// Register the contract with the monitor if it's not already being monitored
	if w.monitor != nil && !w.monitor.IsContractReadyForMonitoring(contract.ID) {
		w.logger.Infof("Registering contract %s with monitor", contract.ID)
		if err := w.monitor.RegisterContract(ctx, contract); err != nil {
			w.logger.Infof("Error registering contract with monitor: %v", err)
			// Continue with backfill even if registration fails
		}
	}

	// Mark the contract as being backfilled
	if w.monitor != nil {
		w.monitor.MarkContractBackfilling(contract.ID)
		w.logger.Infof("Marked contract %s as being backfilled", contract.ID)
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

	w.logger.Infof("Created event listener for contract %s", p.ContractAddr)

	eventChan := make(chan ethereum.Log, 100)
	go func() {
		w.logger.Infof("Starting backfill for contract %s", p.ContractAddr)
		if err := listener.Start(ctx, eventChan); err != nil {
			w.logger.Infof("Error during backfill: %v", err)
		}
		w.logger.Infof("Backfill completed for contract %s", p.ContractAddr)

		// Signal that backfill is complete
		close(done)

		// Mark the contract as having completed backfill
		if w.monitor != nil {
			w.monitor.MarkContractBackfillComplete(contract.ID)
			w.logger.Infof("Marked contract %s as having completed backfill", contract.ID)
		}
	}()

	// Process events from the channel
	for {
		select {
		case <-ctx.Done():
			w.logger.Infof("Context cancelled, stopping backfill for contract %s", p.ContractAddr)

			if w.monitor != nil {
				w.monitor.MarkContractBackfillComplete(contract.ID)
				w.logger.Infof("Marked contract %s as having completed backfill (context cancelled)", contract.ID)
			}
			return nil
		case <-done:
			w.logger.Infof("Backfill process completed for contract %s", p.ContractAddr)
			return nil
		case event, ok := <-eventChan:
			if !ok {
				w.logger.Infof("Event channel closed for contract %s", p.ContractAddr)
				return nil
			}

			decodedData, err := client.GetDecoder().DecodeEvent(p.EventSig, event.Data, event.Topics)
			if err != nil {
				w.logger.Infof("Error decoding event data: %v", err)
				decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(event.Data))
			}

			existingEvent, err := w.db.EventLog.FindFirst(
				db.EventLog.BlockNumber.Equals(int(event.BlockNumber)),
				db.EventLog.TxHash.Equals(strings.ToLower(event.TxHash.Hex())),
				db.EventLog.LogIndex.Equals(int(event.Index)),
			).Exec(ctx)

			if err == nil && existingEvent != nil {
				w.logger.Infof("Event log already exists, skipping: Block=%d, TxHash=%s, Index=%d",
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
				w.logger.Infof("Error storing event: %v", err)
				continue
			}

			w.logger.Infof("Stored new event: Block=%d, TxHash=%s",
				event.BlockNumber, event.TxHash.Hex())
		}
	}
}

func (w *Worker) Shutdown() error {
	w.logger.Infof("Shutting down worker...")
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

	w.logger.Infof("Successfully enqueued task of type %s", taskType)
	return nil
}

// IsContractReadyForMonitoring returns whether a contract is ready for monitoring
func (w *Worker) IsContractReadyForMonitoring(contractID string) bool {
	if w.monitor == nil {
		return false
	}
	return w.monitor.IsContractReadyForMonitoring(contractID)
}
