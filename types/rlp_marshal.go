package types

import (
	"github.com/umbracle/fastrlp"
)

type RLPMarshaler interface {
	MarshalRLPTo(dst []byte) []byte
}

type marshalRLPFunc func(ar *fastrlp.Arena) *fastrlp.Value

func MarshalRLPTo(obj marshalRLPFunc, dst []byte) []byte {
	ar := fastrlp.DefaultArenaPool.Get()
	dst = obj(ar).MarshalTo(dst)
	fastrlp.DefaultArenaPool.Put(ar)

	return dst
}

func (b *Block) MarshalRLP() []byte {
	return b.MarshalRLPTo(nil)
}

func (b *Block) MarshalRLPTo(dst []byte) []byte {
	return MarshalRLPTo(b.MarshalRLPWith, dst)
}

func (b *Block) MarshalRLPWith(ar *fastrlp.Arena) *fastrlp.Value {
	vv := ar.NewArray()
	vv.Set(b.Header.MarshalRLPWith(ar))

	if len(b.Transactions) == 0 {
		vv.Set(ar.NewNullArray())
	} else {
		v0 := ar.NewArray()
		for _, tx := range b.Transactions {
			v0.Set(tx.MarshalRLPWith(ar))
		}
		vv.Set(v0)
	}

	return vv
}

func (h *Header) MarshalRLP() []byte {
	return h.MarshalRLPTo(nil)
}

func (h *Header) MarshalRLPTo(dst []byte) []byte {
	return MarshalRLPTo(h.MarshalRLPWith, dst)
}

// MarshalRLPWith marshals the header to RLP with a specific fastrlp.Arena
func (h *Header) MarshalRLPWith(arena *fastrlp.Arena) *fastrlp.Value {
	vv := arena.NewArray()

	vv.Set(arena.NewBytes(h.ParentHash.Bytes()))
	vv.Set(arena.NewUint(h.ChainId))
	vv.Set(arena.NewBytes(h.StateRoot.Bytes()))
	vv.Set(arena.NewBytes(h.Creator.Bytes()))
	vv.Set(arena.NewUint(h.Number))
	vv.Set(arena.NewUint(h.Timestamp))
	vv.Set(arena.NewBytes(h.ExtraData))
	vv.Set(arena.NewUint(h.SideHead))

	return vv
}

//// MarshalLogsWith marshals the logs of the header to RLP with a specific fastrlp.Arena
//func (r *Header) MarshalLogsWith(a *fastrlp.Arena) *fastrlp.Value {
//	if len(r.Logs) == 0 {
//		// There are no logs, write the RLP null array entry
//		return a.NewNullArray()
//	}
//
//	logs := a.NewArray()
//
//	for _, l := range r.Logs {
//		logs.Set((*Log)(l).MarshalRLPWith(a))
//	}
//
//	return logs
//}
//
//// MarshalFilterWith marshals the filter of the header to RLP with a specific fastrlp.Arena
//func (r *Header) MarshalFilterWith(a *fastrlp.Arena) *fastrlp.Value {
//	v := a.NewArray()
//	if r.Filter != nil {
//		v.Set(a.NewBytes(r.Filter.FromBlock.Bytes()))
//		v.Set(a.NewBytes(r.Filter.ToBlock.Bytes()))
//		v.Set(a.NewBytes(r.Filter.Addresses[0].Bytes()))
//		v.Set(a.NewBytes(r.Filter.BlockHash.Bytes()))
//		// topics
//		topics := a.NewArray()
//		for _, topic := range r.Filter.Topics {
//			topicBytes := a.NewArray()
//			for _, t := range topic {
//				topicBytes.Set(a.NewBytes(t.Bytes()))
//			}
//			topics.Set(topicBytes)
//		}
//		v.Set(topics)
//	}
//	return v
//}

//type Log types.Log
//
//func (l *Log) MarshalRLPWith(a *fastrlp.Arena) *fastrlp.Value {
//	v := a.NewArray()
//	v.Set(a.NewBytes(l.Address.Bytes()))
//
//	topics := a.NewArray()
//	for _, t := range l.Topics {
//		topics.Set(a.NewBytes(t.Bytes()))
//	}
//
//	v.Set(topics)
//	v.Set(a.NewBytes(l.Data))
//	v.Set(a.NewUint(l.BlockNumber))
//	v.Set(a.NewBytes(l.TxHash.Bytes()))
//	v.Set(a.NewUint(uint64(l.TxIndex)))
//	v.Set(a.NewBytes(l.BlockHash.Bytes()))
//	v.Set(a.NewUint(uint64(l.Index)))
//	v.Set(a.NewBool(l.Removed))
//
//	return v
//}

func (t *Transaction) MarshalRLP() []byte {
	return t.MarshalRLPTo(nil)
}

func (t *Transaction) MarshalRLPTo(dst []byte) []byte {
	return MarshalRLPTo(t.MarshalRLPWith, dst)
}

// MarshalRLPWith marshals the transaction to RLP with a specific fastrlp.Arena
func (t *Transaction) MarshalRLPWith(arena *fastrlp.Arena) *fastrlp.Value {
	vv := arena.NewArray()

	vv.Set(arena.NewUint(t.Nonce))
	vv.Set(arena.NewBigInt(t.GasPrice))
	vv.Set(arena.NewUint(t.Gas))

	// Address may be empty
	if t.To != nil {
		vv.Set(arena.NewBytes((*t.To).Bytes()))
	} else {
		vv.Set(arena.NewNull())
	}

	vv.Set(arena.NewBigInt(t.Value))
	vv.Set(arena.NewCopyBytes(t.Input))

	// signature values
	vv.Set(arena.NewBigInt(t.V))
	vv.Set(arena.NewBigInt(t.R))
	vv.Set(arena.NewBigInt(t.S))

	return vv
}
