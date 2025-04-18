package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/umbracle/fastrlp"
)

type RLPUnmarshaler interface {
	UnmarshalRLP(input []byte) error
}

type unmarshalRLPFunc func(p *fastrlp.Parser, v *fastrlp.Value) error

func UnmarshalRlp(obj unmarshalRLPFunc, input []byte) error {
	pr := fastrlp.DefaultParserPool.Get()

	v, err := pr.Parse(input)
	if err != nil {
		fastrlp.DefaultParserPool.Put(pr)

		return err
	}

	if err := obj(pr, v); err != nil {
		fastrlp.DefaultParserPool.Put(pr)

		return err
	}

	fastrlp.DefaultParserPool.Put(pr)

	return nil
}

func (b *Block) UnmarshalRLP(input []byte) error {
	return UnmarshalRlp(b.UnmarshalRLPFrom, input)
}

func (b *Block) UnmarshalRLPFrom(p *fastrlp.Parser, v *fastrlp.Value) error {
	elems, err := v.GetElems()
	if err != nil {
		return err
	}

	if len(elems) < 3 {
		return fmt.Errorf("incorrect number of elements to decode block, expected 3 but found %d", len(elems))
	}

	// header
	b.Header = &Header{}
	if err := b.Header.UnmarshalRLPFrom(p, elems[0]); err != nil {
		return err
	}

	return nil
}

func (h *Header) UnmarshalRLP(input []byte) error {
	return UnmarshalRlp(h.UnmarshalRLPFrom, input)
}

func (h *Header) UnmarshalRLPFrom(p *fastrlp.Parser, v *fastrlp.Value) error {
	elems, err := v.GetElems()
	if err != nil {
		return err
	}

	if len(elems) < 3 {
		return fmt.Errorf("incorrect number of elements to decode header, expected 3 but found %d", len(elems))
	}

	// parentHash
	if err = elems[0].GetHash(h.ParentHash[:]); err != nil {
		return err
	}
	// logs
	// filter

	// compute the hash after the decoding
	h.ComputeHash()

	return err
}

func (l *Log) UnmarshalRLPFrom(p *fastrlp.Parser, v *fastrlp.Value) error {
	elems, err := v.GetElems()
	if err != nil {
		return err
	}

	if len(elems) < 8 {
		return fmt.Errorf("incorrect number of elements to decode log, expected 8 but found %d", len(elems))
	}

	// address
	if err := elems[0].GetAddr(l.Address[:]); err != nil {
		return err
	}
	// topics
	topicElems, err := elems[1].GetElems()
	if err != nil {
		return err
	}

	l.Topics = make([]common.Hash, len(topicElems))

	for indx, topic := range topicElems {
		if err = topic.GetHash(l.Topics[indx][:]); err != nil {
			return err
		}
	}

	// data
	if l.Data, err = elems[2].GetBytes(l.Data[:0]); err != nil {
		return err
	}
	// block number
	if l.BlockNumber, err = elems[3].GetUint64(); err != nil {
		return err
	}
	// tx hash
	if err := elems[4].GetHash(l.TxHash[:]); err != nil {
		return err
	}
	// tx index
	txIndex, err := elems[5].GetUint64()
	if err != nil {
		return err
	}
	l.TxIndex = uint(txIndex)
	// block hash
	if err := elems[6].GetHash(l.BlockHash[:]); err != nil {
		return err
	}
	// index
	index, err := elems[7].GetUint64()
	if err != nil {
		return err
	}
	l.Index = uint(index)
	// removed
	if l.Removed, err = elems[8].GetBool(); err != nil {
		return err
	}

	return nil
}
