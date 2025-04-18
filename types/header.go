package types

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	Hash       Hash                 `json:"hash"`
	ParentHash Hash                 `json:"parentHash"`
	Logs       []*types.Log         `json:"logs"`
	Filter     ethereum.FilterQuery `json:"filter"`
	Creator    Address              `json:"creator"`
}

// headerJSON represents a block header used for json calls
type headerJSON struct {
	Hash       Hash                 `json:"hash"`
	ParentHash Hash                 `json:"parentHash"`
	Logs       []*types.Log         `json:"logs"`
	Filter     ethereum.FilterQuery `json:"filter"`
}

func (h *Header) MarshalJSON() ([]byte, error) {
	var header headerJSON

	header.Hash = h.Hash
	header.ParentHash = h.ParentHash
	header.Logs = h.Logs
	header.Filter = h.Filter

	return json.Marshal(&header)
}

func (h *Header) UnmarshalJSON(input []byte) error {
	var header headerJSON
	if err := json.Unmarshal(input, &header); err != nil {
		return err
	}

	h.Hash = header.Hash
	h.ParentHash = header.ParentHash
	h.Logs = header.Logs
	h.Filter = header.Filter

	return nil
}

func (h *Header) Equal(hh *Header) bool {
	return h.Hash == hh.Hash
}

func (h *Header) Copy() *Header {
	if h == nil {
		return nil
	}

	data, err := h.MarshalJSON()
	if err != nil {
		return nil
	}

	newHeader := new(Header)

	err = newHeader.UnmarshalJSON(data)
	if err != nil {
		return nil
	}

	return newHeader
}

func (h *Header) Number() uint64 {
	return h.Filter.FromBlock.Uint64()
}

type Block struct {
	Header *Header

	// Cache
	size atomic.Value // *uint64
}

func (b *Block) Hash() Hash {
	return b.Header.Hash
}

func (b *Block) ParentHash() Hash {
	return b.Header.ParentHash
}

func (b *Block) Size() uint64 {
	sizePtr := b.size.Load()
	if sizePtr == nil {
		bytes := b.MarshalRLP()
		size := uint64(len(bytes))
		b.size.Store(&size)

		return size
	}

	sizeVal, ok := sizePtr.(*uint64)
	if !ok {
		return 0
	}

	return *sizeVal
}

func (b *Block) String() string {
	str := fmt.Sprintf(`Block(#%v):`, b.Header.Filter.FromBlock)

	return str
}

func (b *Block) Number() uint64 {
	return b.Header.Number()
}
