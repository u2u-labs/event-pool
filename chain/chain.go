package chain

import (
	"event-pool/types"
)

// NodeChain is the event catcher configuration
type NodeChain struct {
	Name      string   `json:"name"`
	Genesis   *Genesis `json:"genesis"`
	Params    *Params  `json:"params"`
	Bootnodes []string `json:"bootnodes,omitempty"`
}

// Genesis specifies state of a genesis block
type Genesis struct {
	Config *Params `json:"config"`

	// Override
	StateRoot types.Hash
}
