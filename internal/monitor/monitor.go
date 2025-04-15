package monitor

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"time"

	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"event-pool/prisma/db"
	pb "event-pool/proto"

	"github.com/ethereum/go-ethereum/common"
)

type Monitor struct {
	ethClients         map[int]*ethereum.Client
	db                 *db.PrismaClient
	grpcServer         *grpc.Server
	mu                 sync.RWMutex
	monitors           map[string]context.CancelFunc
	running            bool
	lastBlocks         map[int]uint64
	backfilling        map[string]bool
	readyForMonitoring map[string]bool
}

func NewMonitor(ethClients map[int]*ethereum.Client, db *db.PrismaClient, grpcServer *grpc.Server) *Monitor {
	return &Monitor{
		ethClients:         ethClients,
		db:                 db,
		grpcServer:         grpcServer,
		monitors:           make(map[string]context.CancelFunc),
		lastBlocks:         make(map[int]uint64),
		backfilling:        make(map[string]bool),
		readyForMonitoring: make(map[string]bool),
	}
}

func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor is already running")
	}
	m.running = true
	m.mu.Unlock()

	fmt.Printf("\n=== Starting Event Monitor ===\n")
	fmt.Printf("Initializing monitor for all registered contracts...\n")

	// Get all contracts from the database
	contracts, err := m.db.Contract.FindMany().Exec(ctx)
	if err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return fmt.Errorf("failed to get contracts: %v", err)
	}

	fmt.Printf("Found %d contracts to monitor\n", len(contracts))

	for chainID, client := range m.ethClients {
		block, err := client.GetLatestBlock()
		if err == nil {
			m.lastBlocks[chainID] = block
			fmt.Printf("Initial block for chain %d: %d\n", chainID, block)
		}
	}

	for _, contract := range contracts {
		contractID := contract.ID

		m.mu.Lock()
		m.readyForMonitoring[contractID] = true
		m.mu.Unlock()

		contractCtx, cancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.monitors[contractID] = cancel
		m.mu.Unlock()

		// Start a goroutine for each contract
		go func(c interface{}) {
			fmt.Printf("Starting independent monitor for contract %s\n", c.(db.ContractModel).ID)
			m.monitorContract(contractCtx, c)
		}(contract)
	}

	// Start a goroutine to periodically log the current block
	go m.logCurrentBlock(ctx)

	// Start a goroutine to periodically check for new contracts
	go m.checkForNewContracts(ctx)

	fmt.Printf("Monitor started successfully. Each contract will be monitored independently.\n")
	fmt.Printf("Contracts being backfilled will be monitored concurrently with backfill.\n")
	fmt.Printf("Will log current blocks every 30 seconds.\n")
	fmt.Printf("Will check for new contracts every 10 seconds.\n")
	fmt.Printf("=== Event Monitor Ready ===\n\n")

	return nil
}

func (m *Monitor) logCurrentBlock(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			if !m.running {
				m.mu.RUnlock()
				return
			}

			// Get the latest block from each chain
			for chainID, client := range m.ethClients {
				block, err := client.GetLatestBlock()
				if err == nil {
					fmt.Printf("Current block on chain %d: %d\n", chainID, block)
				}
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	// Cancel all monitor goroutines
	for _, cancel := range m.monitors {
		cancel()
	}

	// Clear the monitors map and other maps
	m.monitors = make(map[string]context.CancelFunc)
	m.backfilling = make(map[string]bool)
	m.readyForMonitoring = make(map[string]bool)
	m.running = false
	fmt.Printf("Stopped event monitor\n")
}

func (m *Monitor) monitorContract(ctx context.Context, contract interface{}) {
	var chainID int64
	var address string
	var eventSignature string
	var eventABI string
	var id string
	contractValue := reflect.ValueOf(contract)

	if contractValue.Kind() == reflect.Ptr {
		chainID = contractValue.Elem().FieldByName("ChainID").Int()
		address = contractValue.Elem().FieldByName("Address").String()
		eventSignature = contractValue.Elem().FieldByName("EventSignature").String()
		eventABI = contractValue.Elem().FieldByName("EventABI").String()
		id = contractValue.Elem().FieldByName("ID").String()
	} else {
		chainID = contractValue.FieldByName("ChainID").Int()
		address = contractValue.FieldByName("Address").String()
		eventSignature = contractValue.FieldByName("EventSignature").String()
		eventABI = contractValue.FieldByName("EventABI").String()
		id = contractValue.FieldByName("ID").String()
	}

	fmt.Printf("\n=== Starting Monitor for Contract ===\n")
	fmt.Printf("Chain ID: %d\n", chainID)
	fmt.Printf("Address: %s\n", address)
	fmt.Printf("Event Signature: %s\n", eventSignature)
	fmt.Printf("Contract ID: %s\n", id)

	client, ok := m.ethClients[int(chainID)]
	if !ok {
		fmt.Printf("ERROR: No Ethereum client found for chain ID %d\n", chainID)
		return
	}

	// Register the event ABI with the decoder if available
	if eventABI != "" {
		err := client.RegisterEventABI(eventSignature, eventABI)
		if err != nil {
			fmt.Printf("WARNING: Failed to register event ABI: %v\n", err)
		} else {
			fmt.Printf("Successfully registered event ABI for signature %s\n", eventSignature)
		}
	}

	latestBlock, err := client.GetLatestBlock()
	if err != nil {
		fmt.Printf("ERROR: Failed to get latest block: %v\n", err)
		return
	}

	fmt.Printf("Starting to monitor from block %d\n", latestBlock)

	m.mu.Lock()
	if _, exists := m.lastBlocks[int(chainID)]; !exists {
		m.lastBlocks[int(chainID)] = latestBlock
	}
	lastProcessedBlock := m.lastBlocks[int(chainID)]
	m.mu.Unlock()

	fmt.Printf("Last processed block: %d\n", lastProcessedBlock)
	fmt.Printf("=== Monitor Initialized ===\n\n")

	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()

	lastBackfillStatus := m.IsContractBackfilling(id)
	fmt.Printf("Initial backfill status for contract %s: %v\n", id, lastBackfillStatus)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Stopping monitor for contract %s\n", address)
			return
		case <-pollTicker.C:
			currentBackfillStatus := m.IsContractBackfilling(id)
			if currentBackfillStatus != lastBackfillStatus {
				if currentBackfillStatus {
					fmt.Printf("Contract %s is now being backfilled\n", id)
				} else {
					fmt.Printf("Contract %s is no longer being backfilled\n", id)
				}
				lastBackfillStatus = currentBackfillStatus
			}

			currentBlock, err := client.GetLatestBlock()
			if err != nil {
				fmt.Printf("ERROR: Failed to get latest block: %v\n", err)
				continue
			}

			m.mu.Lock()
			lastProcessedBlock := m.lastBlocks[int(chainID)]
			m.mu.Unlock()

			if currentBlock > lastProcessedBlock {
				// fmt.Printf("\n=== Processing New Blocks ===\n")
				// fmt.Printf("Contract: %s\n", address)
				// fmt.Printf("Processing blocks %d to %d\n", lastProcessedBlock+1, currentBlock)

				batchSize := uint64(5)
				for fromBlock := lastProcessedBlock + 1; fromBlock <= currentBlock; fromBlock += batchSize {
					toBlock := fromBlock + batchSize - 1
					if toBlock > currentBlock {
						toBlock = currentBlock
					}

					fmt.Printf("Fetching logs for blocks %d to %d\n", fromBlock, toBlock)

					// Get logs for the block range
					logs, err := client.FilterLogs(
						ctx,
						ethereum.HexToAddress(address),
						ethereum.HexToHash(eventSignature),
						big.NewInt(int64(fromBlock)),
						big.NewInt(int64(toBlock)),
						int(chainID),
					)

					if err != nil {
						fmt.Printf("ERROR: Failed to fetch logs: %v\n", err)
						continue
					}

					fmt.Printf("Found %d logs for blocks %d to %d\n", len(logs), fromBlock, toBlock)

					for _, eventLog := range logs {
						decodedData, err := client.GetDecoder().DecodeEvent(eventSignature, eventLog.Data, eventLog.Topics)
						if err != nil {
							fmt.Printf("ERROR: Failed to decode event data: %v\n", err)
							decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(eventLog.Data))
						}

						err = m.processEvent(ctx, int(chainID), address, eventSignature, eventLog, decodedData)
						if err != nil {
							fmt.Printf("ERROR: Failed to process event: %v\n", err)
							continue
						}
					}

					// Update the last processed block after processing each batch
					m.mu.Lock()
					m.lastBlocks[int(chainID)] = toBlock
					lastProcessedBlock = toBlock
					m.mu.Unlock()

					fmt.Printf("Updated last processed block to %d\n", lastProcessedBlock)
					fmt.Printf("=== Finished Processing Blocks ===\n\n")
				}
			} else {
				fmt.Printf("No new blocks to process. Current: %d, Last: %d\n", currentBlock, lastProcessedBlock)
			}
		}
	}
}

func (m *Monitor) processEvent(ctx context.Context, chainID int, address string, eventSignature string, eventLog ethereum.Log, decodedData interface{}) error {
	// Store event in database
	contract, err := m.db.Contract.FindFirst(
		db.Contract.ChainID.Equals(chainID),
		db.Contract.Address.Equals(strings.ToLower(address)),
		db.Contract.EventSignature.Equals(eventSignature),
	).Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to find contract: %w", err)
	}

	// Create gRPC event
	event := &pb.Event{
		BlockNumber: int64(eventLog.BlockNumber),
		TxHash:      eventLog.TxHash.Hex(),
		Data:        decodedData.(string),
	}

	// Broadcast event via gRPC
	err = m.grpcServer.BroadcastEvent(
		int32(chainID),
		address,
		contract.EventName,
		event,
	)

	if err != nil {
		return fmt.Errorf("failed to broadcast event: %w", err)
	}

	return nil
}

func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Monitor) GetActiveContractCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.monitors)
}

func (m *Monitor) GetLastBlock() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latestBlock int64
	for _, block := range m.lastBlocks {
		if int64(block) > latestBlock {
			latestBlock = int64(block)
		}
	}
	return latestBlock
}

func (m *Monitor) MarkContractBackfilling(contractID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backfilling[contractID] = true

	m.readyForMonitoring[contractID] = true

	fmt.Printf("Marked contract %s as being backfilled\n", contractID)
}

func (m *Monitor) MarkContractBackfillComplete(contractID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backfilling[contractID] = false

	m.readyForMonitoring[contractID] = true

	fmt.Printf("Marked contract %s as having completed backfill\n", contractID)
}

func (m *Monitor) IsContractBackfilling(contractID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	backfilling, exists := m.backfilling[contractID]
	if !exists {
		return false
	}
	return backfilling
}

func (m *Monitor) IsContractReadyForMonitoring(contractID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ready, exists := m.readyForMonitoring[contractID]
	if !exists {
		return false
	}
	return ready
}

func (m *Monitor) RegisterContract(ctx context.Context, contract interface{}) error {
	if !m.IsRunning() {
		return fmt.Errorf("monitor is not running")
	}

	var id string
	contractValue := reflect.ValueOf(contract)

	if contractValue.Kind() == reflect.Ptr {
		id = contractValue.Elem().FieldByName("ID").String()
	} else {
		id = contractValue.FieldByName("ID").String()
	}

	fmt.Printf("\n=== Registering New Contract with Monitor ===\n")
	fmt.Printf("Contract ID: %s\n", id)

	m.mu.Lock()
	m.readyForMonitoring[id] = true
	m.mu.Unlock()

	contractCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.monitors[id] = cancel
	m.mu.Unlock()

	go func(c interface{}) {
		fmt.Printf("Starting independent monitor for newly registered contract %s\n", c.(db.ContractModel).ID)
		m.monitorContract(contractCtx, c)
	}(contract)

	fmt.Printf("Successfully registered contract %s with monitor\n", id)
	fmt.Printf("=== Contract Registration Complete ===\n\n")

	return nil
}

func (m *Monitor) checkForNewContracts(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			if !m.running {
				m.mu.RUnlock()
				return
			}
			m.mu.RUnlock()

			contracts, err := m.db.Contract.FindMany().Exec(ctx)
			if err != nil {
				fmt.Printf("Error getting contracts: %v\n", err)
				continue
			}

			for _, contract := range contracts {
				contractID := contract.ID

				m.mu.RLock()
				_, isMonitored := m.monitors[contractID]
				m.mu.RUnlock()

				if isMonitored {
					continue
				}

				fmt.Printf("Found new contract %s, registering with monitor\n", contractID)
				if err := m.RegisterContract(ctx, contract); err != nil {
					fmt.Printf("Error registering contract %s: %v\n", contractID, err)
				}
			}
		}
	}
}
