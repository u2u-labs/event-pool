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
	StateRoot types.Hash `json:"stateRoot" yaml:"stateRoot"`
}
