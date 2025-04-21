package chain

import (
	"event-pool/types"
)

// NodeChain is the event catcher configuration
type NodeChain struct {
	Name      string   `json:"name" yaml:"name"`
	Genesis   *Genesis `json:"genesis" yaml:"genesis"`
	Params    *Params  `json:"params" yaml:"params"`
	Bootnodes []string `json:"bootnodes,omitempty" yaml:"bootnodes"`
}

// Genesis specifies state of a genesis block
type Genesis struct {
	// Override
	StateRoot  types.Hash `json:"stateRoot" yaml:"stateRoot"`
	Number     uint64     `json:"number" yaml:"number"`
	Timestamp  uint64     `json:"timestamp" yaml:"timestamp"`
	ParentHash types.Hash `json:"parentHash" yaml:"parentHash"`
	ExtraData  []byte     `json:"extraData,omitempty" yaml:"extraData"`
}

// GenesisHeader converts the initially defined genesis struct to a header
func (g *Genesis) GenesisHeader() *types.Header {
	stateRoot := types.EmptyRootHash

	if g.StateRoot != types.ZeroHash {
		stateRoot = g.StateRoot
	}

	head := &types.Header{
		Number:     g.Number,
		Timestamp:  g.Timestamp,
		ParentHash: g.ParentHash,
		StateRoot:  stateRoot,
		ExtraData:  g.ExtraData,
	}

	return head
}
