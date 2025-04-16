package main

import (
	"fmt"
	"os"

	"event-pool/cmd"
	"event-pool/cmd/server"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "event-pool",
	Short: "Event Pool - Ethereum event listener and publisher service",
	Long: `Event Pool is a service that listens to specific smart contract events 
on supported L1 chains, stores them, and pushes them to subscribing clients in real-time.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the event pool service",
	RunE:  cmd.RunServe,
}

func init() {
	rootCmd.AddCommand(
		serveCmd,
		server.GetCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
