package main

import (
	"fmt"
	"os"

	"event-pool/cmd"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "event-pool",
	Short: "Event Pool - Ethereum event listener and publisher service",
	Long: `Event Pool is a service that listens to specific smart contract events 
on supported L1 chains, stores them, and pushes them to subscribing clients in real-time.`,
}

// Define the commands in main.go
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the event pool service",
	RunE:  cmd.RunServe,
}

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Backfill historical events for a contract",
	RunE:  cmd.RunBackfill,
}

func init() {
	// Set up flags for backfill command
	cmd.SetupBackfillFlags(backfillCmd)

	// Add commands here
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(backfillCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
