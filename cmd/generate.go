package cmd

import (
	"fmt"

	"event-pool/internal/keyutil"
	"event-pool/p2p"
	"github.com/spf13/cobra"
)

var (
	TargetPath = ""
)

func Generate(cmd *cobra.Command, args []string) error {
	logger := p2p.SetupLogger()

	privKey, _, err := keyutil.GenerateKeyPair()
	if err != nil {
		logger.Fatalf("Failed to generate key pair: %v", err)
	}
	if TargetPath == "" {
		TargetPath = "./data/tmp/key"
	}

	_ = keyutil.SavePrivateKey(privKey, TargetPath)
	privKeyBytes, _ := privKey.Raw()

	// save pub key
	pubKeyBytes, _ := privKey.GetPublic().Raw()
	_ = keyutil.SavePublicKey(privKey.GetPublic(), TargetPath+".pub")

	logger.Infow("Key info", "privateKey", fmt.Sprintf("%x", privKeyBytes), "publicKey", fmt.Sprintf("%x", pubKeyBytes))

	return nil
}
