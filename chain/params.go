package chain

import (
	"event-pool/types"
)

// Params are all the set of params for the chain
type Params struct {
	ChainIDs   []int       `json:"chain_ids" yaml:"chain_ids"`
	Whitelists *Whitelists `json:"whitelists,omitempty" yaml:"whitelists"`
}

// Whitelists specifies supported whitelists
type Whitelists struct {
	Deployment []types.Address `json:"deployment,omitempty" yaml:"deployment"`
}
