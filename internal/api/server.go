package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"event-pool/internal/config"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/websocket"
	"event-pool/prisma/db"
)

type Server struct {
	config     *config.Config
	db         *db.PrismaClient
	worker     *worker.Worker
	wsServer   *websocket.Server
	ethClients map[int]*ethereum.Client
	monitor    *monitor.Monitor
}

// MonitorStatus represents the current status of the monitor
type MonitorStatus struct {
	IsRunning       bool
	ActiveContracts int
	LastBlock       int64
}

// GetMonitorStatus returns the current status of the monitor
func (s *Server) GetMonitorStatus() MonitorStatus {
	return MonitorStatus{
		IsRunning:       s.monitor != nil && s.monitor.IsRunning(),
		ActiveContracts: s.monitor.GetActiveContractCount(),
		LastBlock:       s.monitor.GetLastBlock(),
	}
}

// GetActiveContracts returns a list of contracts currently being monitored
func (s *Server) GetActiveContracts(ctx context.Context) ([]db.ContractModel, error) {
	return s.db.Contract.FindMany().Exec(ctx)
}

func NewServer(config *config.Config, db *db.PrismaClient, worker *worker.Worker, ethClients map[int]*ethereum.Client) *Server {
	// Create WebSocket server
	wsConfig := &websocket.Config{
		PingInterval:   30 * time.Second,
		PongWait:       60 * time.Second,
		WriteWait:      10 * time.Second,
		MaxMessageSize: 512 * 1024, // 512KB
	}
	wsServer := websocket.NewServer(wsConfig)

	// Create monitor
	mon := monitor.NewMonitor(ethClients, db, wsServer)

	return &Server{
		config:     config,
		db:         db,
		worker:     worker,
		wsServer:   wsServer,
		ethClients: ethClients,
		monitor:    mon,
	}
}

// StartMonitor starts the event monitor
func (s *Server) StartMonitor() error {
	if s.monitor == nil {
		return fmt.Errorf("monitor not initialized")
	}
	return s.monitor.Start(context.Background())
}

// Stop stops the server and its components
func (s *Server) Stop() {
	if s.monitor != nil {
		s.monitor.Stop()
		log.Println("Monitor stopped")
	}
}

func (s *Server) Start() error {
	// Start the monitor first
	if err := s.StartMonitor(); err != nil {
		return fmt.Errorf("failed to start monitor: %w", err)
	}
	log.Printf("Monitor started successfully")

	// Create handlers
	contractHandler := NewContractHandler(s.db, s.worker, s.config, s.ethClients)

	// Set up routes
	http.HandleFunc("/api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			contractHandler.RegisterContract(w, r)
		case http.MethodGet:
			contractHandler.GetContracts(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Add monitor status endpoint
	http.HandleFunc("/api/v1/monitor/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := s.GetMonitorStatus()
		fmt.Fprintf(w, "Monitor Status:\nRunning: %v\nActive Contracts: %d\nLast Block: %d\n",
			status.IsRunning, status.ActiveContracts, status.LastBlock)
	})

	// WebSocket endpoint for event subscriptions
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a WebSocket request
		if websocket.IsWebSocketUpgrade(r) {
			// Parse path: /{chainId}/{contractAddress}/{eventId}
			path := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.Split(path, "/")

			if len(parts) != 3 {
				http.Error(w, "Invalid WebSocket path format. Expected: /{chainId}/{contractAddress}/{eventId}", http.StatusBadRequest)
				return
			}

			// Validate the contract exists in the database
			chainID, err := strconv.Atoi(parts[0])
			if err != nil {
				http.Error(w, "Invalid chain ID", http.StatusBadRequest)
				return
			}

			contractAddr := parts[1]
			eventID := parts[2]

			// Check if the contract exists in the database
			contract, err := s.db.Contract.FindUnique(
				db.Contract.ID.Equals(eventID),
			).Exec(context.Background())

			if err != nil {
				log.Printf("Contract not found with ID: %s", eventID)
				http.Error(w, "Contract not found", http.StatusNotFound)
				return
			}

			// Verify that the contract matches the chain ID and address
			if contract.ChainID != chainID || strings.ToLower(contract.Address) != strings.ToLower(contractAddr) {
				log.Printf("Contract mismatch: ChainID=%d, Address=%s, EventID=%s", chainID, contractAddr, eventID)
				http.Error(w, "Contract mismatch", http.StatusBadRequest)
				return
			}

			log.Printf("WebSocket connection for contract: %s", contract.ID)
			s.wsServer.HandleWebSocket(w, r)
			return
		}

		// Default response for non-WebSocket requests
		fmt.Fprintf(w, "Event Pool API Server")
	})

	// Start the server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}
