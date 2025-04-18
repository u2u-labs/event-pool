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

	logFileLocation string
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
	p.rawConfig.GRPCAddr = grpcAddress
}

func (p *serverParams) setRawJSONRPCAddress(jsonRPCAddress string) {
	p.rawConfig.JSONRPCAddr = jsonRPCAddress
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
		DataDir:        p.rawConfig.DataDir,
		SecretsManager: p.secretsConfig,
		LogLevel:       lvl,
		DbUrl:          p.rawConfig.Database.Url,
	}
}
