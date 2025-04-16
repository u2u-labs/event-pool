package server

import (
	"net"

	"event-pool/chain"
	"event-pool/network"
	"event-pool/secrets"
	"go.uber.org/zap/zapcore"
)

const DefaultGRPCPort int = 9632

const DefaultJSONRPCPort int = 8545

// Config is used to parametrize the minimal client
type Config struct {
	Chain *chain.Chain

	JSONRPC    *JSONRPC
	GRPCAddr   *net.TCPAddr
	LibP2PAddr *net.TCPAddr

	Telemetry *Telemetry
	Network   *network.Config

	DataDir string

	SecretsManager *secrets.SecretsManagerConfig

	LogLevel zapcore.Level
}

// Telemetry holds the config details for metric services
type Telemetry struct {
	PrometheusAddr *net.TCPAddr
}

// JSONRPC holds the config details for the JSON-RPC server
type JSONRPC struct {
	JSONRPCAddr              *net.TCPAddr
	AccessControlAllowOrigin []string
	BatchLengthLimit         uint64
	BlockRangeLimit          uint64
}
