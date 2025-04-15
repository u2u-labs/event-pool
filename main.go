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

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the event pool service",
	RunE:  cmd.RunServe,
}

var runNodeCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a node in the event pool network",
	RunE:  cmd.RunNode,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new private key",
	RunE:  cmd.Generate,
}

func init() {
	rootCmd.AddCommand(serveCmd, runNodeCmd, generateCmd)
	runNodeCmd.Flags().StringVar(&cmd.PrivateKeyPath, "private-key", "", "Path to the private key file")
	generateCmd.Flags().StringVar(&cmd.TargetPath, "target-path", "", "Path to save the generated private key")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
