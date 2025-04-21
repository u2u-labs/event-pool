package chain

import (
	"encoding/json"

	"event-pool/types"
)

// NodeChain is the event catcher configuration
type NodeChain struct {
	Name               string        `json:"name" yaml:"name"`
	Genesis            *Genesis      `json:"genesis" yaml:"genesis"`
	Params             *Params       `json:"params" yaml:"params"`
	Bootnodes          []string      `json:"bootnodes,omitempty" yaml:"bootnodes"`
	RpcInfo            *RpcInfo      `json:"rpc_info" yaml:"rpc_info"`
	NodeStorageAddress types.Address `json:"node_storage_address" yaml:"node_storage_address"`
}

type RpcInfo struct {
	RpcUrl    string `json:"rpc_url" yaml:"rpc_url"`
	BlockTime int64  `json:"block_time" yaml:"block_time"`
}

// Genesis specifies state of a genesis block
type Genesis struct {
	// Override
	StateRoot  types.Hash `json:"stateRoot" yaml:"stateRoot"`
	Number     uint64     `json:"number" yaml:"number"`
	Timestamp  uint64     `json:"timestamp" yaml:"timestamp"`
	ParentHash types.Hash `json:"parentHash" yaml:"parentHash"`
	ExtraData  []byte     `json:"extraData,omitempty" yaml:"extraData"`
	ChainId    uint64     `json:"chainId" yaml:"chainId"`
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
		ChainId:    g.ChainId,
	}

	return head
}

func (n *NodeChain) Clone() *NodeChain {
	if n == nil {
		return nil
	}

	// Marshal the original struct to JSON
	jsonData, err := json.Marshal(n)
	if err != nil {
		return nil
	}

	clone := &NodeChain{}

	// Unmarshal the JSON data into the new instance
	err = json.Unmarshal(jsonData, clone)
	if err != nil {
		return nil
	}

	return clone
}
