package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"event-pool/executor"
	"event-pool/internal/config"
	"event-pool/network"
	"event-pool/network/secrets"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/spf13/cobra"
)

var (
	PrivateKeyPath = ""
)

func RunNode(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	logger := network.SetupLogger()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load private key if provided
	var privKey crypto.PrivKey
	if PrivateKeyPath != "" {
		privKey, err = secrets.LoadPrivateKeyFromPath(PrivateKeyPath)
		if err != nil {
			logger.Fatalf("Failed to load private key: %v", err)
		}
	} else if cfg.Node.PrivateKeyPath != "" {
		privKey, err = secrets.LoadPrivateKeyFromPath(cfg.Node.PrivateKeyPath)
		if err != nil {
			logger.Fatalf("Failed to load private key: %v", err)
		}
	} else {
		logger.Info("No private key provided, generating a new one...")
		privKey, _, err = secrets.GenerateKeyPair()
		if err != nil {
			logger.Fatalf("Failed to generate key pair: %v", err)
		}
		_ = secrets.SavePrivateKey(privKey, "./data/private_key.pem")
		privKeyBytes, _ := privKey.Raw()
		logger.Infow("Key info", "privateKey", fmt.Sprintf("%x", privKeyBytes))
	}

	// Create whitelist manager
	whitelist := secrets.NewWhitelistManager()

	// Add whitelisted public keys
	for _, path := range cfg.Node.WhitelistPaths {
		if err = whitelist.AddPublicKeyFromFile(path); err != nil {
			logger.Warnf("Failed to add whitelisted key: %v\n", err)
		}
	}

	e := executor.NewContractExecutor(logger)

	// Create node with options
	node, err := network.NewChainNode(
		ctx,
		nil,
		e,
		network.WithPrivateKey(privKey),
		network.WithWhitelist(whitelist),
	)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Rest of node startup logic
	defer node.Close()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Node started. Press Ctrl+C to stop.")
	// Keep node running
	select {
	case <-sigChan:
	case <-ctx.Done():
	}
	logger.Info("Shutting down...")

	return nil
}
