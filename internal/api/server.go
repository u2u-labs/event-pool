package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

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

func NewServer(config *config.Config, db *db.PrismaClient, worker *worker.Worker, ethClients map[int]*ethereum.Client) *Server {
	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create monitor
	mon := monitor.NewMonitor(ethClients, db, grpcServer)

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
		log.Println("gRPC server stopped")
	}
}

func (s *Server) Start() error {
	// Start the gRPC server
	go func() {
		if err := s.grpcServer.Start(s.config.GRPC.Port); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Start the monitor
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

	// Start the HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, nil)
}
