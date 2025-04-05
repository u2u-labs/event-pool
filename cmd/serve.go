package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"event-pool/internal/api"
	"event-pool/internal/config"
	"event-pool/internal/db"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/websocket"

	"github.com/spf13/cobra"
)

// RunServe is the function that runs the serve command
func RunServe(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	dbClient, err := db.NewClient()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close(dbClient)

	// Initialize Ethereum clients
	ethClients := make(map[int]*ethereum.Client)
	for chainID, chainConfig := range cfg.Ethereum.Chains {
		client, err := ethereum.NewClient(chainConfig.RPCURL, chainID, chainConfig.BlockTime, dbClient)
		if err != nil {
			return fmt.Errorf("failed to initialize Ethereum client for chain %d: %w", chainID, err)
		}
		ethClients[chainID] = client
	}

	// Initialize WebSocket server
	wsConfig := &websocket.Config{
		PingInterval:   30,
		PongWait:       60,
		WriteWait:      10,
		MaxMessageSize: 512,
	}
	wsServer := websocket.NewServer(wsConfig)

	// Initialize monitor
	mon := monitor.NewMonitor(ethClients, dbClient, wsServer)

	// Initialize worker
	worker := worker.NewWorker(cfg.Asynq.RedisAddr, ethClients, dbClient, mon)
	go func() {
		if err := worker.Start(); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	// Initialize API server
	server := api.NewServer(cfg, dbClient, worker, ethClients)
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Graceful shutdown
	if err := worker.Shutdown(); err != nil {
		log.Printf("Error shutting down worker: %v", err)
	}

	return nil
}
