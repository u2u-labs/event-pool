package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"event-pool/internal/config"
	"event-pool/internal/jwt"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"event-pool/prisma/db"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Server struct {
	config     *config.Config
	db         *db.PrismaClient
	rdb        *redis.Client
	worker     *worker.Worker
	grpcServer *grpc.Server
	ethClients map[int]*ethereum.Client
	monitor    *monitor.Monitor
	logger     *zap.SugaredLogger
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

func NewServer(config *config.Config, db *db.PrismaClient, rdb *redis.Client, worker *worker.Worker, ethClients map[int]*ethereum.Client, grpcServer *grpc.Server, mon *monitor.Monitor, logger *zap.SugaredLogger) *Server {
	// grpcServer := grpc.NewServer()
	// Create monitor
	// mon := monitor.NewMonitor(ethClients, db, grpcServer)

	return &Server{
		config:     config,
		db:         db,
		rdb:        rdb,
		worker:     worker,
		grpcServer: grpcServer,
		ethClients: ethClients,
		monitor:    mon,
		logger:     logger,
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
		s.logger.Infoln("Monitor stopped")
	}
	if s.grpcServer != nil {
		s.grpcServer.Stop()
		s.logger.Infoln("GRPC server stopped")
	}
}

func (s *Server) Start() error {
	// Start the monitor first
	if err := s.StartMonitor(); err != nil {
		return fmt.Errorf("failed to start monitor: %w", err)
	}
	s.logger.Infof("Monitor started successfully")

	go func() {
		if err := s.grpcServer.Start(s.config.GrpcServer.Port); err != nil {
			s.logger.Errorf("Failed to start grpc server: %v", err)
		}
	}()
	s.logger.Infof("gRPC server started successfully on port %d", s.config.GrpcServer.Port)

	// Create handlers
	contractHandler := NewContractHandler(s.db, s.worker, s.config, s.ethClients, s.logger.Named("contract"))

	httpMux := mux.NewRouter()
	httpMux.Use(s.loggingMiddleware)
	httpMux.Use(allowCORS)

	authRoutes := httpMux.NewRoute().Subrouter()
	authRoutes.Use(authMiddleware(s.config.JwtSecret))
	authRoutes.Use(s.successBasedRateLimitMiddleware(100000, time.Hour)) // 100000 requests per hour

	// Set up routes
	httpMux.HandleFunc("/api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
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
	authRoutes.HandleFunc("/api/v1/events", contractHandler.GetEvents).Methods(http.MethodGet)

	// Fix the incomplete handler
	httpMux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "API endpoint not found", http.StatusNotFound)
	})

	// Add monitor status endpoint
	httpMux.HandleFunc("/api/v1/monitor/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := s.GetMonitorStatus()
		fmt.Fprintf(w, "Monitor Status:\nRunning: %v\nActive Contracts: %d\nLast Block: %d\n",
			status.IsRunning, status.ActiveContracts, status.LastBlock)
	})

	// Add MQTT subscription endpoint
	httpMux.HandleFunc("/api/v1/subscribe", func(w http.ResponseWriter, r *http.Request) {
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

	httpMux.HandleFunc("/api/v1/token", s.grpcServer.RequestTokenHandler)
	httpMux.HandleFunc("/api/v1/ws", s.grpcServer.HandleWs)
	httpMux.HandleFunc("/api/v1/disconnect", s.grpcServer.DisconnectWs)
	httpMux.HandleFunc("/api/v1/status/{chainId}/{address}/{eventName}", s.grpcServer.GetContractStatus).Methods("GET")
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// with cors
	srv := &http.Server{
		Handler:           httpMux,
		ReadHeaderTimeout: 60 * time.Second,
	}

	// Start the server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.logger.Infof("Starting server on %s", addr)
	return srv.Serve(lis)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		s.logger.Infow("incoming request",
			"remote", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
			"duration", duration,
		)
	})
}

func allowCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		h.ServeHTTP(w, r)
	})
}

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			claims, err := jwt.ValidateJWT(authHeader, []byte(jwtSecret), nil)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "address", strings.ToLower(claims.Address))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
