package server

import (
	"errors"
	"net"

	"event-pool/chain"
	"event-pool/cmd/server/config"
	"event-pool/network"
	"event-pool/secrets"
	"event-pool/server"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap/zapcore"
)

const (
	configFlag            = "config"
	dataDirFlag           = "data-dir"
	libp2pAddressFlag     = "libp2p"
	prometheusAddressFlag = "prometheus"
	natFlag               = "nat"
	dnsFlag               = "dns"
	maxPeersFlag          = "max-peers"
	maxInboundPeersFlag   = "max-inbound-peers"
	maxOutboundPeersFlag  = "max-outbound-peers"
	secretsConfigFlag     = "secrets-config"
	devIntervalFlag       = "dev-interval"
	devFlag               = "dev"
	corsOriginFlag        = "access-control-allow-origins"
)

// Flags that are deprecated, but need to be preserved for
// backwards compatibility with existing scripts
const (
	ibftBaseTimeoutFlagLEGACY = "ibft-base-timeout"
)

const (
	unsetPeersValue = -1
)

var (
	params = &serverParams{
		rawConfig: &config.Config{
			Telemetry: &config.Telemetry{},
			Network:   &config.Network{},
		},
	}
)

var (
	errInvalidNATAddress = errors.New("could not parse NAT IP address")
)

type serverParams struct {
	rawConfig  *config.Config
	configPath string

	libp2pAddress     *net.TCPAddr
	prometheusAddress *net.TCPAddr
	natAddress        net.IP
	dnsAddress        multiaddr.Multiaddr
	grpcAddress       *net.TCPAddr
	jsonRPCAddress    *net.TCPAddr

	devInterval uint64
	isDevMode   bool

	corsAllowedOrigins []string

	ibftBaseTimeoutLegacy uint64

	genesisConfig *chain.NodeChain
	secretsConfig *secrets.SecretsManagerConfig
}

func (p *serverParams) isMaxPeersSet() bool {
	return p.rawConfig.Network.MaxPeers != unsetPeersValue
}

func (p *serverParams) isPeerRangeSet() bool {
	return p.rawConfig.Network.MaxInboundPeers != unsetPeersValue ||
		p.rawConfig.Network.MaxOutboundPeers != unsetPeersValue
}

func (p *serverParams) isSecretsConfigPathSet() bool {
	return p.rawConfig.SecretsConfigPath != ""
}

func (p *serverParams) isPrometheusAddressSet() bool {
	return p.rawConfig.Telemetry.PrometheusAddr != ""
}

func (p *serverParams) isNATAddressSet() bool {
	return p.rawConfig.Network.NatAddr != ""
}

func (p *serverParams) isDNSAddressSet() bool {
	return p.rawConfig.Network.DNSAddr != ""
}

func (p *serverParams) setRawGRPCAddress(grpcAddress string) {
	if grpcAddress == "" {
		return
	}
	p.rawConfig.GRPCAddr = grpcAddress
}

func (p *serverParams) setRawJSONRPCAddress(jsonRPCAddress string) {
	if jsonRPCAddress == "" {
		return
	}
	p.rawConfig.JSONRPCAddr = jsonRPCAddress
}

func (p *serverParams) setRawLibp2p(libp2p string) {
	if libp2p == "" {
		return
	}
	p.rawConfig.LibP2PAddr = libp2p
	p.rawConfig.Network.Libp2pAddr = libp2p
}

func (p *serverParams) setRawDataDir(dataDir string) {
	if dataDir == "" {
		return
	}
	p.rawConfig.DataDir = dataDir
}

func (p *serverParams) setRawPrometheus(prometheusAddr string) {
	if prometheusAddr == "" {
		return
	}
	p.rawConfig.Telemetry.PrometheusAddr = prometheusAddr
}

func (p *serverParams) generateConfig() *server.Config {
	lvl, _ := zapcore.ParseLevel(p.rawConfig.LogLevel)
	return &server.Config{
		Chain: p.rawConfig.NodeChain,
		JSONRPC: &server.JSONRPC{
			JSONRPCAddr:              p.jsonRPCAddress,
			AccessControlAllowOrigin: p.corsAllowedOrigins,
		},
		GRPCAddr:   p.grpcAddress,
		LibP2PAddr: p.libp2pAddress,
		Telemetry: &server.Telemetry{
			PrometheusAddr: p.prometheusAddress,
		},
		Network: &network.Config{
			NoDiscover:       p.rawConfig.Network.NoDiscover,
			Addr:             p.libp2pAddress,
			NatAddr:          p.natAddress,
			DNS:              p.dnsAddress,
			DataDir:          p.rawConfig.DataDir,
			MaxPeers:         p.rawConfig.Network.MaxPeers,
			MaxInboundPeers:  p.rawConfig.Network.MaxInboundPeers,
			MaxOutboundPeers: p.rawConfig.Network.MaxOutboundPeers,
			Chain:            p.rawConfig.NodeChain,
		},
		DataDir:            p.rawConfig.DataDir,
		SecretsManager:     p.secretsConfig,
		LogLevel:           lvl,
		DbUrl:              p.rawConfig.Database.Url,
		BlockTime:          p.rawConfig.NodeChain.Params.BlockTime,
		PriceLimit:         p.rawConfig.NodeChain.Params.PriceLimit,
		MaxAccountEnqueued: p.rawConfig.NodeChain.Params.MaxAccountEnqueued,
		MaxSlots:           p.rawConfig.NodeChain.Params.MaxSlots,
		EpochSize:          p.rawConfig.NodeChain.Params.EpochSize,
		EthereumRpc:        p.rawConfig.EthereumRpc,
		NodeStorageAddress: p.rawConfig.NodeStorageAddress,
		MonitorApiPort:     p.rawConfig.MonitorConfig.Port,
		MonitorApiHost:     p.rawConfig.MonitorConfig.Host,
		RedisConfig: &server.RedisConfig{
			Addr:     p.rawConfig.RedisConfig.Addr,
			Password: p.rawConfig.RedisConfig.Password,
			DB:       p.rawConfig.RedisConfig.DB,
		},
	}
}
