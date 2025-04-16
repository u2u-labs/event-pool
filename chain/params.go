package chain

import (
	"event-pool/types"
)

// Params are all the set of params for the chain
type Params struct {
	ChainID    int         `json:"chain_id"`
	Whitelists *Whitelists `json:"whitelists,omitempty"`
}

// Whitelists specifies supported whitelists
type Whitelists struct {
	Deployment []types.Address `json:"deployment,omitempty"`
}
