package types

import (
	"fmt"

	"github.com/umbracle/fastrlp"
)

type RLPStoreUnmarshaler interface {
	UnmarshalStoreRLP(input []byte) error
}

func (b *Body) UnmarshalRLP(input []byte) error {
	return UnmarshalRlp(b.UnmarshalRLPFrom, input)
}

func (b *Body) UnmarshalRLPFrom(p *fastrlp.Parser, v *fastrlp.Value) error {
	tuple, err := v.GetElems()
	if err != nil {
		return err
	}

	if len(tuple) < 1 {
		return fmt.Errorf("incorrect number of elements to decode header, expected 1 but found %d", len(tuple))
	}

	// transactions
	txns, err := tuple[0].GetElems()
	if err != nil {
		return err
	}

	for _, txn := range txns {
		bTxn := &Transaction{}
		if err := bTxn.UnmarshalStoreRLPFrom(p, txn); err != nil {
			return err
		}

		b.Transactions = append(b.Transactions, bTxn)
	}

	return nil
}

func (t *Transaction) UnmarshalStoreRLP(input []byte) error {
	return UnmarshalRlp(t.UnmarshalStoreRLPFrom, input)
}

func (t *Transaction) UnmarshalStoreRLPFrom(p *fastrlp.Parser, v *fastrlp.Value) error {
	elems, err := v.GetElems()
	if err != nil {
		return err
	}

	if len(elems) < 2 {
		return fmt.Errorf("incorrect number of elements to decode transaction, expected 2 but found %d", len(elems))
	}

	// consensus part
	if err := t.UnmarshalRLPFrom(p, elems[0]); err != nil {
		return err
	}
	// context part
	if err = elems[1].GetAddr(t.From[:]); err != nil {
		return err
	}

	return nil
}
