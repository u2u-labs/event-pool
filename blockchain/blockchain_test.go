package blockchain

import (
	"fmt"

	"event-pool/types"
)

type dummyChain struct {
	headers map[byte]*types.Header
}

func (c *dummyChain) add(h *header) error {
	if _, ok := c.headers[h.hash]; ok {
		return fmt.Errorf("hash already imported")
	}

	var parent types.Hash
	if h.number != 0 {
		p, ok := c.headers[h.parent]
		if !ok {
			return fmt.Errorf("parent not found %v", h.parent)
		}

		parent = p.Hash
	}

	hh := &types.Header{
		ParentHash: parent,
	}

	hh.ComputeHash()
	c.headers[h.hash] = hh

	return nil
}

type header struct {
	hash   byte
	parent byte
	number uint64
	diff   uint64
}

func (h *header) Parent(parent byte) *header {
	h.parent = parent
	h.number = uint64(parent) + 1

	return h
}

func (h *header) Diff(d uint64) *header {
	h.diff = d

	return h
}

func (h *header) Number(d uint64) *header {
	h.number = d

	return h
}

func mock(number byte) *header {
	return &header{
		hash:   number,
		parent: number - 1,
		number: uint64(number),
		diff:   uint64(number),
	}
}
