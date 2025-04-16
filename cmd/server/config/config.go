package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"event-pool/chain"
	"event-pool/network"
	"github.com/hashicorp/hcl"
	"gopkg.in/yaml.v3"
)

// Config defines the server configuration params
type Config struct {
	SecretsConfigPath string           `json:"secrets_config" yaml:"secrets_config"`
	DataDir           string           `json:"data_dir" yaml:"data_dir"`
	GRPCAddr          string           `json:"grpc_addr" yaml:"grpc_addr"`
	JSONRPCAddr       string           `json:"jsonrpc_addr" yaml:"jsonrpc_addr"`
	Telemetry         *Telemetry       `json:"telemetry" yaml:"telemetry"`
	Network           *Network         `json:"network" yaml:"network"`
	LogLevel          string           `json:"log_level" yaml:"log_level"`
	NodeChain         *chain.NodeChain `json:"node_chain" yaml:"node_chain"`
	Database          *Database        `json:"database" yaml:"database"`
}

// Telemetry holds the config details for metric services.
type Telemetry struct {
	PrometheusAddr string `json:"prometheus_addr" yaml:"prometheus_addr"`
}

// Network defines the network configuration params
type Network struct {
	NoDiscover       bool   `json:"no_discover" yaml:"no_discover"`
	Libp2pAddr       string `json:"libp2p_addr" yaml:"libp2p_addr"`
	NatAddr          string `json:"nat_addr" yaml:"nat_addr"`
	DNSAddr          string `json:"dns_addr" yaml:"dns_addr"`
	MaxPeers         int64  `json:"max_peers,omitempty" yaml:"max_peers,omitempty"`
	MaxOutboundPeers int64  `json:"max_outbound_peers,omitempty" yaml:"max_outbound_peers,omitempty"`
	MaxInboundPeers  int64  `json:"max_inbound_peers,omitempty" yaml:"max_inbound_peers,omitempty"`
}

type Database struct {
	Url string `json:"url" yaml:"url"`
}

// Headers defines the HTTP response headers required to enable CORS.
type Headers struct {
	AccessControlAllowOrigins []string `json:"access_control_allow_origins" yaml:"access_control_allow_origins"`
}

// DefaultConfig returns the default server configuration
func DefaultConfig() *Config {
	defaultNetworkConfig := network.DefaultConfig()

	return &Config{
		DataDir: "",
		Network: &Network{
			NoDiscover:       defaultNetworkConfig.NoDiscover,
			MaxPeers:         defaultNetworkConfig.MaxPeers,
			MaxOutboundPeers: defaultNetworkConfig.MaxOutboundPeers,
			MaxInboundPeers:  defaultNetworkConfig.MaxInboundPeers,
			Libp2pAddr: fmt.Sprintf("%s:%d",
				defaultNetworkConfig.Addr.IP,
				defaultNetworkConfig.Addr.Port,
			),
		},
		Telemetry: &Telemetry{},
		LogLevel:  "INFO",
	}
}

// ReadConfigFile reads the config file from the specified path, builds a Config object
// and returns it.
//
// Supported file types: .json, .hcl, .yaml, .yml
func ReadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var unmarshalFunc func([]byte, interface{}) error

	switch {
	case strings.HasSuffix(path, ".hcl"):
		unmarshalFunc = hcl.Unmarshal
	case strings.HasSuffix(path, ".json"):
		unmarshalFunc = json.Unmarshal
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		unmarshalFunc = yaml.Unmarshal
	default:
		return nil, fmt.Errorf("suffix of %s is neither hcl, json, yaml nor yml", path)
	}

	config := DefaultConfig()
	config.Network = new(Network)
	config.Network.MaxPeers = -1
	config.Network.MaxInboundPeers = -1
	config.Network.MaxOutboundPeers = -1

	if err := unmarshalFunc(data, config); err != nil {
		return nil, err
	}

	return config, nil
}
