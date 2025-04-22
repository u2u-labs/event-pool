package chain

import (
	"event-pool/types"
)

// Params are all the set of params for the chain
type Params struct {
	Forks      *Forks      `json:"forks" yaml:"forks"`
	ChainID    int         `json:"chain_id" yaml:"chain_id"`
	Whitelists *Whitelists `json:"whitelists,omitempty" yaml:"whitelists"`
}

// Whitelists specifies supported whitelists
type Whitelists struct {
	Deployment []types.Address `json:"deployment,omitempty" yaml:"deployment"`
}

type ForksInTime struct {
	Homestead,
	Byzantium,
	Constantinople,
	Petersburg,
	Istanbul,
	EIP150,
	EIP158,
	EIP155 bool
}

// Forks specifies when each fork is activated
type Forks struct {
	Homestead      *Fork `json:"homestead,omitempty"`
	Byzantium      *Fork `json:"byzantium,omitempty"`
	Constantinople *Fork `json:"constantinople,omitempty"`
	Petersburg     *Fork `json:"petersburg,omitempty"`
	Istanbul       *Fork `json:"istanbul,omitempty"`
	EIP150         *Fork `json:"EIP150,omitempty"`
	EIP158         *Fork `json:"EIP158,omitempty"`
	EIP155         *Fork `json:"EIP155,omitempty"`
}

type Fork uint64

func (f *Forks) At(block uint64) ForksInTime {
	return ForksInTime{
		Homestead:      f.active(f.Homestead, block),
		Byzantium:      f.active(f.Byzantium, block),
		Constantinople: f.active(f.Constantinople, block),
		Petersburg:     f.active(f.Petersburg, block),
		Istanbul:       f.active(f.Istanbul, block),
		EIP150:         f.active(f.EIP150, block),
		EIP158:         f.active(f.EIP158, block),
		EIP155:         f.active(f.EIP155, block),
	}
}

func (f *Forks) active(ff *Fork, block uint64) bool {
	if ff == nil {
		return false
	}

	return ff.Active(block)
}

func (f Fork) Active(block uint64) bool {
	return block >= uint64(f)
}
