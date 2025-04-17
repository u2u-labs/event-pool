package server

import (
	"fmt"

	cmd2 "event-pool/cmd"
	"event-pool/cmd/helper"
	"event-pool/cmd/server/config"
	"event-pool/server"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "server",
		Short:   "The default command that starts the server",
		PreRunE: runPreRun,
		Run:     runCommand,
	}

	helper.RegisterGRPCAddressFlag(serverCmd)
	helper.RegisterLegacyGRPCAddressFlag(serverCmd)
	helper.RegisterJSONRPCFlag(serverCmd)

	registerSubcommands(serverCmd)
	setFlags(serverCmd)

	return serverCmd
}

func registerSubcommands(baseCmd *cobra.Command) {
	baseCmd.AddCommand()
}

func setFlags(cmd *cobra.Command) {
	defaultConfig := config.DefaultConfig()

	cmd.Flags().StringVar(
		&params.rawConfig.LogLevel,
		cmd2.LogLevelFlag,
		defaultConfig.LogLevel,
		"the log level for console output",
	)

	cmd.Flags().StringVar(
		&params.configPath,
		configFlag,
		"config.yaml",
		"the path to the CLI config. Supports .yaml, .hcl and .json files",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.DataDir,
		dataDirFlag,
		defaultConfig.DataDir,
		"the data directory used for storing the keys",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.Network.Libp2pAddr,
		libp2pAddressFlag,
		defaultConfig.Network.Libp2pAddr,
		"the address and port for the libp2p service",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.Telemetry.PrometheusAddr,
		prometheusAddressFlag,
		"",
		"the address and port for the prometheus instrumentation service (address:port). "+
			"If only port is defined (:port) it will bind to 0.0.0.0:port",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.Network.NatAddr,
		natFlag,
		"",
		"the external IP address without port, as can be seen by peers",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.Network.DNSAddr,
		dnsFlag,
		"",
		"the host DNS address which can be used by a remote peer for connection",
	)

	cmd.Flags().StringVar(
		&params.rawConfig.SecretsConfigPath,
		secretsConfigFlag,
		"",
		"the path to the SecretsManager config file. Used for Hashicorp Vault. "+
			"If omitted, the local FS secrets manager is used",
	)

	cmd.Flags().BoolVar(
		&params.rawConfig.Network.NoDiscover,
		cmd2.NoDiscoverFlag,
		defaultConfig.Network.NoDiscover,
		"prevent the client from discovering other peers",
	)

	cmd.Flags().Int64Var(
		&params.rawConfig.Network.MaxPeers,
		maxPeersFlag,
		-1,
		"the client's max number of peers allowed",
	)
	// override default usage value
	cmd.Flag(maxPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxPeers)

	cmd.Flags().Int64Var(
		&params.rawConfig.Network.MaxInboundPeers,
		maxInboundPeersFlag,
		-1,
		"the client's max number of inbound peers allowed",
	)
	// override default usage value
	cmd.Flag(maxInboundPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxInboundPeers)
	cmd.MarkFlagsMutuallyExclusive(maxPeersFlag, maxInboundPeersFlag)

	cmd.Flags().Int64Var(
		&params.rawConfig.Network.MaxOutboundPeers,
		maxOutboundPeersFlag,
		-1,
		"the client's max number of outbound peers allowed",
	)
	// override default usage value
	cmd.Flag(maxOutboundPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxOutboundPeers)
	cmd.MarkFlagsMutuallyExclusive(maxPeersFlag, maxOutboundPeersFlag)

	setLegacyFlags(cmd)

	setDevFlags(cmd)
}

// setLegacyFlags sets the legacy flags to preserve backwards compatibility
// with running partners
func setLegacyFlags(cmd *cobra.Command) {
	// Legacy IBFT base timeout flag
	cmd.Flags().Uint64Var(
		&params.ibftBaseTimeoutLegacy,
		ibftBaseTimeoutFlagLEGACY,
		0,
		"",
	)

	_ = cmd.Flags().MarkHidden(ibftBaseTimeoutFlagLEGACY)
}

func setDevFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&params.isDevMode,
		devFlag,
		false,
		"should the client start in dev mode (default false)",
	)

	_ = cmd.Flags().MarkHidden(devFlag)

	cmd.Flags().Uint64Var(
		&params.devInterval,
		devIntervalFlag,
		0,
		"the client's dev notification interval in seconds (default 1)",
	)

	_ = cmd.Flags().MarkHidden(devIntervalFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	// Set the grpc and json ip:port bindings
	// The config file will have precedence over --flag
	params.setRawGRPCAddress(helper.GetGRPCAddress(cmd))
	params.setRawJSONRPCAddress(helper.GetJSONRPCAddress(cmd))

	// Check if the config file has been specified
	// Config file settings will override JSON-RPC and GRPC address values
	if isConfigFileSpecified(cmd) {
		if err := params.initConfigFromFile(); err != nil {
			return err
		}
	}

	if err := params.initRawParams(); err != nil {
		return err
	}

	return nil
}

func isConfigFileSpecified(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(configFlag)
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := cmd2.InitializeOutputter(cmd)

	if err := runServerLoop(params.generateConfig(), outputter); err != nil {
		outputter.SetError(err)
		outputter.WriteOutput()
		return
	}
}

func runServerLoop(
	config *server.Config,
	outputter cmd2.OutputFormatter,
) error {
	serverInstance, err := server.NewServer(config)
	if err != nil {
		return err
	}

	return helper.HandleSignals(serverInstance.Close, outputter)
}
