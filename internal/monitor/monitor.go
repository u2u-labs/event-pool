package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	crypto2 "event-pool/crypto"
	pb "event-pool/internal/proto"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"event-pool/prisma/db"
	"event-pool/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"

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
	logger             *zap.SugaredLogger
}

func NewMonitor(ethClients map[int]*ethereum.Client, db *db.PrismaClient, grpcServer *grpc.Server, logger *zap.SugaredLogger) *Monitor {
	return &Monitor{
		ethClients:         ethClients,
		db:                 db,
		grpcServer:         grpcServer,
		monitors:           make(map[string]context.CancelFunc),
		lastBlocks:         make(map[int]uint64),
		backfilling:        make(map[string]bool),
		readyForMonitoring: make(map[string]bool),
		logger:             logger,
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

	m.logger.Infof("\n=== Starting Event Monitor ===\n")
	m.logger.Infof("Initializing monitor for all registered contracts...\n")

	// Get all contracts from the database
	contracts, err := m.db.Contract.FindMany().Exec(ctx)
	if err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return fmt.Errorf("failed to get contracts: %v", err)
	}

	m.logger.Infof("Found %d contracts to monitor\n", len(contracts))

	for chainID, client := range m.ethClients {
		block, err := client.GetLatestBlock()
		if err == nil {
			m.lastBlocks[chainID] = block
			m.logger.Infof("Initial block for chain %d: %d\n", chainID, block)
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
			m.logger.Infof("Starting independent monitor for contract %s\n", c.(db.ContractModel).ID)
			m.monitorContract(contractCtx, c)
		}(contract)
	}

	// Start a goroutine to periodically log the current block
	go m.logCurrentBlock(ctx)

	// Start a goroutine to periodically check for new contracts
	go m.checkForNewContracts(ctx)

	m.logger.Infof("Monitor started successfully. Each contract will be monitored independently.\n")
	m.logger.Infof("Contracts being backfilled will be monitored concurrently with backfill.\n")
	m.logger.Infof("Will log current blocks every 30 seconds.\n")
	m.logger.Infof("Will check for new contracts every 10 seconds.\n")
	m.logger.Infof("=== Event Monitor Ready ===\n\n")

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
					m.logger.Infof("Current block on chain %d: %d\n", chainID, block)
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
	m.logger.Infof("Stopped event monitor\n")
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

	m.logger.Infof("\n=== Starting Monitor for Contract ===\n")
	m.logger.Infof("Chain ID: %d\n", chainID)
	m.logger.Infof("Address: %s\n", address)
	m.logger.Infof("Event Signature: %s\n", eventSignature)
	m.logger.Infof("Contract ID: %s\n", id)

	client, ok := m.ethClients[int(chainID)]
	if !ok {
		m.logger.Infof("ERROR: No Ethereum client found for chain ID %d\n", chainID)
		return
	}

	// Register the event ABI with the decoder if available
	if eventABI != "" {
		err := client.RegisterEventABI(eventSignature, eventABI)
		if err != nil {
			m.logger.Infof("WARNING: Failed to register event ABI: %v\n", err)
		} else {
			m.logger.Infof("Successfully registered event ABI for signature %s\n", eventSignature)
		}
	}

	latestBlock, err := client.GetLatestBlock()
	if err != nil {
		m.logger.Infof("ERROR: Failed to get latest block: %v\n", err)
		return
	}

	m.logger.Infof("Starting to monitor from block %d\n", latestBlock)

	m.mu.Lock()
	if _, exists := m.lastBlocks[int(chainID)]; !exists {
		m.lastBlocks[int(chainID)] = latestBlock
	}
	lastProcessedBlock := m.lastBlocks[int(chainID)]
	m.mu.Unlock()

	m.logger.Infof("Last processed block: %d\n", lastProcessedBlock)
	m.logger.Infof("=== Monitor Initialized ===\n\n")

	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()

	lastBackfillStatus := m.IsContractBackfilling(id)
	m.logger.Infof("Initial backfill status for contract %s: %v\n", id, lastBackfillStatus)

	for {
		select {
		case <-ctx.Done():
			m.logger.Infof("Stopping monitor for contract %s\n", address)
			return
		case <-pollTicker.C:
			currentBackfillStatus := m.IsContractBackfilling(id)
			if currentBackfillStatus != lastBackfillStatus {
				if currentBackfillStatus {
					m.logger.Infof("Contract %s is now being backfilled\n", id)
				} else {
					m.logger.Infof("Contract %s is no longer being backfilled\n", id)
				}
				lastBackfillStatus = currentBackfillStatus
			}

			currentBlock, err := client.GetLatestBlock()
			if err != nil {
				m.logger.Infof("ERROR: Failed to get latest block: %v\n", err)
				continue
			}

			m.mu.Lock()
			lastProcessedBlock := m.lastBlocks[int(chainID)]
			m.mu.Unlock()

			if currentBlock > lastProcessedBlock {
				// m.logger.Infof()("\n=== Processing New Blocks ===\n")
				// m.logger.Infof()("Contract: %s\n", address)
				// m.logger.Infof()("Processing blocks %d to %d\n", lastProcessedBlock+1, currentBlock)

				batchSize := uint64(5)
				for fromBlock := lastProcessedBlock + 1; fromBlock <= currentBlock; fromBlock += batchSize {
					toBlock := fromBlock + batchSize - 1
					if toBlock > currentBlock {
						toBlock = currentBlock
					}

					m.logger.Infof("Fetching logs for blocks %d to %d\n", fromBlock, toBlock)

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
						m.logger.Infof("ERROR: Failed to fetch logs: %v\n", err)
						continue
					}

					m.logger.Infof("Found %d logs for blocks %d to %d\n", len(logs), fromBlock, toBlock)

					m.ProcessLogsEvent(ctx, logs, client, eventSignature, chainID, address)

					if len(logs) > 0 {
						// add query logs as txn to the chain
						err = m.SendTx(ctx, types.FilterLogsParams{
							FromBlock:       big.NewInt(int64(fromBlock)),
							ToBlock:         big.NewInt(int64(toBlock)),
							ContractAddress: ethereum.HexToAddress(address),
							EventSignature:  ethereum.HexToHash(eventSignature),
							ChainId:         int(chainID),
						})
						if err != nil {
							m.logger.Infof("ERROR: Failed to send transaction: %v\n", err)
						}
					}

					// Update the last processed block after processing each batch
					m.mu.Lock()
					m.lastBlocks[int(chainID)] = toBlock
					lastProcessedBlock = toBlock
					m.mu.Unlock()

					m.logger.Infof("Updated last processed block to %d\n", lastProcessedBlock)
					m.logger.Infof("=== Finished Processing Blocks ===\n\n")
				}
			} else {
				m.logger.Infof("No new blocks to process. Current: %d, Last: %d\n", currentBlock, lastProcessedBlock)
			}
		}
	}
}

func (m *Monitor) ProcessLogsEvent(ctx context.Context, logs []ethereum.Log, client *ethereum.Client, eventSignature string, chainID int64, address string) {
	for _, eventLog := range logs {
		decodedData, err := client.GetDecoder().DecodeEvent(eventSignature, eventLog.Data, eventLog.Topics)
		if err != nil {
			m.logger.Infof("ERROR: Failed to decode event data: %v\n", err)
			decodedData = fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(eventLog.Data))
		}

		err = m.processEvent(ctx, int(chainID), address, eventSignature, eventLog, decodedData)
		if err != nil {
			m.logger.Infof("ERROR: Failed to process event: %v\n", err)
			continue
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

	if m.grpcServer == nil {
		// If gRPC server is not running, just return
		return nil
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

	m.logger.Infof("Marked contract %s as being backfilled\n", contractID)
}

func (m *Monitor) MarkContractBackfillComplete(contractID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backfilling[contractID] = false

	m.readyForMonitoring[contractID] = true

	m.logger.Infof("Marked contract %s as having completed backfill\n", contractID)
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

	m.logger.Infof("\n=== Registering New Contract with Monitor ===\n")
	m.logger.Infof("Contract ID: %s\n", id)

	m.mu.Lock()
	m.readyForMonitoring[id] = true
	m.mu.Unlock()

	contractCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.monitors[id] = cancel
	m.mu.Unlock()

	go func(c interface{}, id string) {
		m.logger.Infof("Starting independent monitor for newly registered contract %s\n", id)
		m.monitorContract(contractCtx, c)
	}(contract, id)

	m.logger.Infof("Successfully registered contract %s with monitor\n", id)
	m.logger.Infof("=== Contract Registration Complete ===\n\n")

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
				m.logger.Infof("Error getting contracts: %v\n", err)
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

				m.logger.Infof("Found new contract %s, registering with monitor\n", contractID)
				if err := m.RegisterContract(ctx, contract); err != nil {
					m.logger.Infof("Error registering contract %s: %v\n", contractID, err)
				}
			}
		}
	}
}

func (m *Monitor) SendTx(ctx context.Context, filter types.FilterLogsParams) error {
	filter.ComputeHash()
	input, err := json.Marshal(filter)
	if err != nil {
		return err
	}

	// placeholder to address
	addr := types.StringToAddress("0x01857E2BCFcb8B4eF76Df6590F8dCd3bf736C9E9")
	tx := &types.Transaction{
		Nonce:    0,
		GasPrice: big.NewInt(1000000000),
		Gas:      21000,
		To:       &addr,
		Value:    big.NewInt(1000000000000000000),
		Input:    input,
	}

	secretBytes, err := os.ReadFile(filepath.Join(viper.GetString("data_dir"), "consensus/validator.key"))
	if err != nil {
		return err
	}
	priv, err := crypto2.BytesToECDSAPrivateKey(secretBytes)
	if err != nil {
		return err
	}

	rawBytes, err := ethereum.SignTransaction(tx, priv)
	if err != nil {
		return err
	}

	dataBytes := fmt.Sprintf("0x%x", rawBytes)
	bodyData := map[string]string{
		"data": dataBytes,
		"from": "",
	}
	payload, err := json.Marshal(bodyData)
	if err != nil {
		return err
	}

	// send post request to the server
	resp, err := http.Post(
		fmt.Sprintf("http://%s%s/txpool/add",
			viper.GetString("JSONRPC_HOST"),
			viper.GetString("jsonrpc_addr")),
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send transaction: %s", resp.Status)
	}

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// parse response body
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	m.logger.Infof("Transaction ID: %s\n", response["txHash"])

	return nil
}
