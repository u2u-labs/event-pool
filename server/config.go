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
	Chain *chain.NodeChain

	JSONRPC    *JSONRPC
	GRPCAddr   *net.TCPAddr
	LibP2PAddr *net.TCPAddr

	Telemetry *Telemetry
	Network   *network.Config

	DataDir string

	SecretsManager *secrets.SecretsManagerConfig

	LogLevel zapcore.Level

	DbUrl              string
	BlockTime          uint64
	PriceLimit         uint64
	MaxAccountEnqueued uint64
	MaxSlots           uint64
	EpochSize          uint64

	EthereumRpc        *EthereumRpc
	NodeStorageAddress string
	MonitorApiPort     string
	MonitorApiHost     string
	RedisConfig        *RedisConfig
}

type EthereumRpc struct {
	Chains map[int]chain.RpcInfo `json:"chains" yaml:"chains"`
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

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}
