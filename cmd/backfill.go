package cmd

import (
	"github.com/spf13/cobra"
)

var (
	chainId      int
	contractAddr string
	eventSig     string
	startBlock   int
)

// RunBackfill is the function that runs the backfill command
func RunBackfill(cmd *cobra.Command, args []string) error {
	// TODO: Implement backfill logic
	// 1. Validate chain ID is supported
	// 2. Create asynq task for backfilling
	return nil
}

// SetupBackfillFlags sets up the flags for the backfill command
func SetupBackfillFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&chainId, "chain-id", 0, "Chain ID to backfill")
	cmd.Flags().StringVar(&contractAddr, "contract", "", "Contract address to backfill")
	cmd.Flags().StringVar(&eventSig, "event", "", "Event signature to backfill")
	cmd.Flags().IntVar(&startBlock, "start-block", 0, "Starting block number")

	cmd.MarkFlagRequired("chain-id")
	cmd.MarkFlagRequired("contract")
	cmd.MarkFlagRequired("event")
	cmd.MarkFlagRequired("start-block")
}
