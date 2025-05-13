package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"event-pool/internal/api"
	"event-pool/internal/config"
	"event-pool/internal/db"
	"event-pool/internal/monitor"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/pkg/grpc"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/spf13/cobra"
)

// RunServe is the function that runs the serve command
func RunServe(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	zapConfig := zap.NewDevelopmentConfig()

	lvl, _ := zapcore.ParseLevel(cfg.LogLevel)
	zapConfig.Level = zap.NewAtomicLevelAt(lvl)
	zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	zLogger, err := zapConfig.Build()
	if err != nil {
		panic(err) // or handle gracefully
	}
	logger := zLogger.Sugar()

	// Initialize redis
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection
	if _, err = client.Ping(ctx).Result(); err != nil {
		return err
	}

	jwtSecret, err := os.ReadFile(cfg.JwtSecretPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read JWT secret: %w", err)
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
		client, err := ethereum.NewClient(chainConfig.RPCURL, chainID, chainConfig.BlockTime, dbClient, logger.Named("rpc"))
		if err != nil {
			return fmt.Errorf("failed to initialize Ethereum client for chain %d: %w", chainID, err)
		}
		ethClients[chainID] = client
	}

	// Initialize gRPC server
	grpcServer := grpc.NewServer(dbClient, cfg.SecretKey, strings.TrimSpace(string(jwtSecret)), cfg.SessionContract, cfg.NodeContract, ethClients[cfg.ChainId], client, logger.Named("server"))

	// Initialize monitor
	mon := monitor.NewMonitor(ethClients, dbClient, grpcServer, logger.Named("monitor"))

	// Initialize worker
	worker := worker.NewWorker(cfg.Asynq.RedisAddr, ethClients, dbClient, mon, logger.Named("worker"))
	go func() {
		if err := worker.Start(); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	// Initialize API server
	server := api.NewServer(cfg, dbClient, worker, ethClients, grpcServer, mon, logger.Named("api"))
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Infoln("Shutting down...")

	// Graceful shutdown
	server.Stop()
	return nil
}
