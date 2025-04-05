package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"event-pool/internal/config"
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

	return &Server{
		config:     config,
		db:         db,
		worker:     worker,
		wsServer:   wsServer,
		ethClients: ethClients,
	}
}

func (s *Server) Start() error {
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

	// WebSocket endpoint for event subscriptions
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a WebSocket request
		if websocket.IsWebSocketUpgrade(r) {
			// Parse path: /{chainId}/{contractAddress}/{event}
			path := strings.TrimPrefix(r.URL.Path, "/")
			parts := strings.Split(path, "/")

			if len(parts) != 3 {
				http.Error(w, "Invalid WebSocket path format. Expected: /{chainId}/{contractAddress}/{event}", http.StatusBadRequest)
				return
			}

			// TODO: Validate the contract exists in the database
			// TODO: Set up the WebSocket connection

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
