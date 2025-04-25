package types

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	Hash       Hash    `json:"hash"`
	ParentHash Hash    `json:"parentHash"`
	ChainId    uint64  `json:"chainId"`
	StateRoot  Hash    `json:"stateRoot"`
	Creator    Address `json:"creator"`
	Number     uint64  `json:"number"`
	Timestamp  uint64  `json:"timestamp"`
	ExtraData  []byte  `json:"extraData"`
}

// headerJSON represents a block header used for json calls
type headerJSON struct {
	Hash       Hash    `json:"hash"`
	ParentHash Hash    `json:"parentHash"`
	ChainId    uint64  `json:"chainId"`
	StateRoot  Hash    `json:"stateRoot"`
	Creator    Address `json:"creator"`
	Number     uint64  `json:"number"`
	Timestamp  uint64  `json:"timestamp"`
	ExtraData  []byte  `json:"extraData"`
}

func (h *Header) MarshalJSON() ([]byte, error) {
	var header headerJSON

	header.Hash = h.Hash
	header.ParentHash = h.ParentHash
	header.ChainId = h.ChainId
	header.StateRoot = h.StateRoot
	header.Creator = h.Creator
	header.Number = h.Number
	header.Timestamp = h.Timestamp
	header.ExtraData = h.ExtraData

	return json.Marshal(&header)
}

func (h *Header) UnmarshalJSON(input []byte) error {
	var header headerJSON
	if err := json.Unmarshal(input, &header); err != nil {
		return err
	}

	h.Hash = header.Hash
	h.ParentHash = header.ParentHash
	h.ChainId = header.ChainId
	h.StateRoot = header.StateRoot
	h.Creator = header.Creator
	h.Number = header.Number
	h.Timestamp = header.Timestamp
	h.ExtraData = header.ExtraData

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

type Block struct {
	Header *Header

	Transactions []*Transaction
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
	str := fmt.Sprintf(`Block(#%v):`, b.Header.Number)

	return str
}

func (b *Block) Body() *Body {
	return &Body{
		Transactions: b.Transactions,
	}
}

func (b *Block) Number() uint64 {
	return b.Header.Number
}

type Body struct {
	Transactions []*Transaction
}
