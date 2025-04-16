package cmd

import (
	"fmt"

	"event-pool/network"
	"event-pool/network/secrets"
	"github.com/spf13/cobra"
)

var (
	TargetPath = ""
)

func Generate(cmd *cobra.Command, args []string) error {
	logger := network.SetupLogger()

	privKey, _, err := secrets.GenerateKeyPair()
	if err != nil {
		logger.Fatalf("Failed to generate key pair: %v", err)
	}
	if TargetPath == "" {
		TargetPath = "./data/tmp/key"
	}

	_ = secrets.SavePrivateKey(privKey, TargetPath)
	privKeyBytes, _ := privKey.Raw()

	// save pub key
	pubKeyBytes, _ := privKey.GetPublic().Raw()
	_ = secrets.SavePublicKey(privKey.GetPublic(), TargetPath+".pub")

	logger.Infow("Key info", "privateKey", fmt.Sprintf("%x", privKeyBytes), "publicKey", fmt.Sprintf("%x", pubKeyBytes))

	return nil
}
