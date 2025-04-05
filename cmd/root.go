package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "event-pool",
	Short: "Event Pool - Ethereum event monitoring system",
}

func Execute() error {
	return rootCmd.Execute()
}
