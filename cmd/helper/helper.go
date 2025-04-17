package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"event-pool/chain"
	cmd2 "event-pool/cmd"
	"event-pool/helper/common"
	"event-pool/server"
	"event-pool/server/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientCloseResult struct {
	Message string `json:"message"`
}

func (r *ClientCloseResult) GetOutput() string {
	return r.Message
}

type IPBinding string

const (
	LocalHostBinding     IPBinding = "127.0.0.1"
	AllInterfacesBinding IPBinding = "0.0.0.0"
)

// HandleSignals is a helper method for handling signals sent to the console
// Like stop, error, etc.
func HandleSignals(
	closeFn func(),
	outputter cmd2.OutputFormatter,
) error {
	signalCh := common.GetTerminationSignalCh()
	sig := <-signalCh

	closeMessage := fmt.Sprintf("\n[SIGNAL] Caught signal: %v\n", sig)
	closeMessage += "Gracefully shutting down client...\n"

	outputter.SetCommandResult(
		&ClientCloseResult{
			Message: closeMessage,
		},
	)
	outputter.WriteOutput()

	// Call the Minimal server close callback
	gracefulCh := make(chan struct{})

	go func() {
		if closeFn != nil {
			closeFn()
		}

		close(gracefulCh)
	}()

	select {
	case <-signalCh:
		return errors.New("shutdown by signal channel")
	case <-time.After(5 * time.Second):
		return errors.New("shutdown by timeout")
	case <-gracefulCh:
		return nil
	}
}

// GetSystemClientConnection returns the System operator client connection
func GetSystemClientConnection(address string) (
	proto.SystemClient,
	error,
) {
	conn, err := GetGRPCConnection(address)
	if err != nil {
		return nil, err
	}

	return proto.NewSystemClient(conn), nil
}

// GetGRPCConnection returns a grpc client connection
func GetGRPCConnection(address string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return conn, nil
}

// GetGRPCAddress extracts the set GRPC address
func GetGRPCAddress(cmd *cobra.Command) string {
	if cmd.Flags().Changed(cmd2.GRPCAddressFlagLEGACY) {
		// The legacy GRPC flag was set, use that value
		return cmd.Flag(cmd2.GRPCAddressFlagLEGACY).Value.String()
	}

	return cmd.Flag(cmd2.GRPCAddressFlag).Value.String()
}

// GetJSONRPCAddress extracts the set JSON-RPC address
func GetJSONRPCAddress(cmd *cobra.Command) string {
	return cmd.Flag(cmd2.JSONRPCFlag).Value.String()
}

// RegisterJSONOutputFlag registers the --json output setting for all child commands
func RegisterJSONOutputFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(
		cmd2.JSONOutputFlag,
		false,
		"get all outputs in json format (default false)",
	)
}

// RegisterGRPCAddressFlag registers the base GRPC address flag for all child commands
func RegisterGRPCAddressFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String(
		cmd2.GRPCAddressFlag,
		fmt.Sprintf("%s:%d", LocalHostBinding, server.DefaultGRPCPort),
		"the GRPC interface",
	)
}

// RegisterLegacyGRPCAddressFlag registers the legacy GRPC address flag for all child commands
func RegisterLegacyGRPCAddressFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String(
		cmd2.GRPCAddressFlagLEGACY,
		fmt.Sprintf("%s:%d", LocalHostBinding, server.DefaultGRPCPort),
		"the GRPC interface",
	)

	// Mark the legacy grpc flag as hidden
	_ = cmd.PersistentFlags().MarkHidden(cmd2.GRPCAddressFlagLEGACY)
}

// ParseGRPCAddress parses the passed in GRPC address
func ParseGRPCAddress(grpcAddress string) (*net.TCPAddr, error) {
	return net.ResolveTCPAddr("tcp", grpcAddress)
}

// RegisterJSONRPCFlag registers the base JSON-RPC address flag for all child commands
func RegisterJSONRPCFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String(
		cmd2.JSONRPCFlag,
		fmt.Sprintf("%s:%d", AllInterfacesBinding, server.DefaultJSONRPCPort),
		"the JSON-RPC interface",
	)
}

// ParseJSONRPCAddress parses the passed in JSONRPC address
func ParseJSONRPCAddress(jsonrpcAddress string) (*url.URL, error) {
	return url.ParseRequestURI(jsonrpcAddress)
}

// ResolveAddr resolves the passed in TCP address
// The second param is the default ip to bind to, if no ip address is specified
func ResolveAddr(address string, defaultIP IPBinding) (*net.TCPAddr, error) {
	addr, err := net.ResolveTCPAddr("tcp", address)

	if err != nil {
		return nil, fmt.Errorf("failed to parse addr '%s': %w", address, err)
	}

	if addr.IP == nil {
		addr.IP = net.ParseIP(string(defaultIP))
	}

	return addr, nil
}

// WriteGenesisConfigToDisk writes the passed in configuration to a genesis file at the specified path
func WriteGenesisConfigToDisk(genesisConfig *chain.NodeChain, genesisPath string) error {
	data, err := json.MarshalIndent(genesisConfig, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to generate genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, data, os.ModePerm); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	return nil
}

func SetRequiredFlags(cmd *cobra.Command, requiredFlags []string) {
	for _, requiredFlag := range requiredFlags {
		_ = cmd.MarkFlagRequired(requiredFlag)
	}
}
