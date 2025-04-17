package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"event-pool/internal/config"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"event-pool/prisma/db"

	"github.com/hashicorp/consul/api"
)

type Server struct {
	config       *config.Config
	db           *db.PrismaClient
	worker       *worker.Worker
	grpcServer   *grpc.Server
	ethClients   map[int]*ethereum.Client
	monitor      *monitor.Monitor
	httpServer   *http.Server
	consulClient *api.Client
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

// Start starts the server and its components
func (s *Server) Start() error {
	if s == nil {
		return fmt.Errorf("server is not initialized")
	}

	// Start the gRPC server first
	go func() {
		log.Printf("Starting gRPC server on port %d", s.config.GRPC.Port)
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

	// Add health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start the HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	log.Printf("Starting HTTP server on %s", addr)

	// Create a new server with timeouts
	s.httpServer = &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Register with Consul
	if err := s.registerWithConsul(); err != nil {
		return fmt.Errorf("failed to register with Consul: %w", err)
	}

	return nil
}

// registerWithConsul registers the service with Consul
func (s *Server) registerWithConsul() error {
	// Create Consul client
	consulConfig := api.DefaultConfig()
	consulConfig.Address = s.config.GetConsulAddr()
	consulClient, err := api.NewClient(consulConfig)
	if err != nil {
		return fmt.Errorf("failed to create Consul client: %w", err)
	}
	s.consulClient = consulClient

	// Create tags for each chain this node handles
	var tags []string
	for chainID := range s.config.Ethereum.Chains {
		tags = append(tags, fmt.Sprintf("chain:%d", chainID))
	}

	// Create registration
	registration := &api.AgentServiceRegistration{
		ID:      s.config.Consul.ID,
		Name:    s.config.Consul.ServiceID,
		Port:    s.config.GRPC.Port,
		Address: s.config.Server.Address,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("%v:%d/health", s.config.Server.Address, s.config.Consul.HealthCheck.Port),
			Interval: s.config.Consul.HealthCheck.Interval,
			Timeout:  s.config.Consul.HealthCheck.Timeout,
		},
		Tags: tags,
	}

	if err := s.consulClient.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	fmt.Printf("Successfully registered service with Consul: %s, chains: %v", s.config.Consul.ServiceID, tags)
	return nil
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
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		} else {
			log.Println("HTTP server stopped")
		}
	}
	if s.consulClient != nil {
		if err := s.consulClient.Agent().ServiceDeregister(s.config.Consul.ID); err != nil {
			log.Printf("Failed to deregister service from Consul: %v", err)
		} else {
			log.Println("Service deregistered from Consul")
		}
	}
}
