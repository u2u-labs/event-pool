package types

import "github.com/umbracle/fastrlp"

type RLPStoreMarshaler interface {
	MarshalStoreRLPTo(dst []byte) []byte
}

func (b *Body) MarshalRLPTo(dst []byte) []byte {
	return MarshalRLPTo(b.MarshalRLPWith, dst)
}

func (b *Body) MarshalRLPWith(ar *fastrlp.Arena) *fastrlp.Value {
	vv := ar.NewArray()
	if len(b.Transactions) == 0 {
		vv.Set(ar.NewNullArray())
	} else {
		v0 := ar.NewArray()
		for _, tx := range b.Transactions {
			v0.Set(tx.MarshalStoreRLPWith(ar))
		}
		vv.Set(v0)
	}

	return vv
}

func (t *Transaction) MarshalStoreRLPTo(dst []byte) []byte {
	return MarshalRLPTo(t.MarshalStoreRLPWith, dst)
}

func (t *Transaction) MarshalStoreRLPWith(a *fastrlp.Arena) *fastrlp.Value {
	vv := a.NewArray()
	// consensus part
	vv.Set(t.MarshalRLPWith(a))
	// context part
	vv.Set(a.NewBytes(t.From.Bytes()))

	return vv
}
