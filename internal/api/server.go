package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"event-pool/internal/config"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"event-pool/prisma/db"
)

type Server struct {
	config     *config.Config
	db         *db.PrismaClient
	worker     *worker.Worker
	grpcServer *grpc.Server
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

func NewServer(config *config.Config, db *db.PrismaClient, worker *worker.Worker, ethClients map[int]*ethereum.Client, grpcServer *grpc.Server, mon *monitor.Monitor) *Server {
	// grpcServer := grpc.NewServer()
	// Create monitor
	// mon := monitor.NewMonitor(ethClients, db, grpcServer)

	return &Server{
		config:     config,
		db:         db,
		worker:     worker,
		grpcServer: grpcServer,
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
	if s.grpcServer != nil {
		s.grpcServer.Stop()
		log.Println("GRPC server stopped")
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

	// events query endpoint
	http.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		contractHandler.GetEvents(w, r)
	})

	// Fix the incomplete handler
	http.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "API endpoint not found", http.StatusNotFound)
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

	// Add MQTT subscription endpoint
	http.HandleFunc("/api/v1/subscribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req struct {
			ChainID        int    `json:"chainId"`
			ContractAddr   string `json:"contractAddress"`
			EventSignature string `json:"eventSignature"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate contract exists
		_, err := s.db.Contract.FindFirst(
			db.Contract.ChainID.Equals(req.ChainID),
			db.Contract.Address.Equals(strings.ToLower(req.ContractAddr)),
			db.Contract.EventSignature.Equals(req.EventSignature),
		).Exec(r.Context())

		if err != nil {
			http.Error(w, "Contract not found", http.StatusNotFound)
			return
		}

		// Register the topic for tracking
		//s.grpcServer.RegisterTopic(req.ChainID, req.ContractAddr, req.EventSignature)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "registered",
			"topic":  fmt.Sprintf("events/%d/%s/%s", req.ChainID, req.ContractAddr, req.EventSignature),
		})
	})

	// Start the server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}
