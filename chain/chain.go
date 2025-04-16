package chain

import (
	"event-pool/types"
)

// Chain is the blockchain chain configuration
type Chain struct {
	Name      string   `json:"name"`
	Genesis   *Genesis `json:"genesis"`
	Params    *Params  `json:"params"`
	Bootnodes []string `json:"bootnodes,omitempty"`
}

// Genesis specifies the header fields, state of a genesis block
type Genesis struct {
	Config *Params `json:"config"`

	// Override
	StateRoot types.Hash
}
