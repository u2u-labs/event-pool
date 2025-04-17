package cmd

const (
	ConsensusFlag  = "consensus"
	NoDiscoverFlag = "no-discover"
	BootnodeFlag   = "bootnode"
	LogLevelFlag   = "log-level"
)

const (
	DefaultChainID = 100

	JSONOutputFlag  = "json"
	GRPCAddressFlag = "grpc-address"
	JSONRPCFlag     = "jsonrpc"
)

// GRPCAddressFlagLEGACY Legacy flag that needs to be present to preserve backwards
// compatibility with running clients
const (
	GRPCAddressFlagLEGACY = "grpc"
)
