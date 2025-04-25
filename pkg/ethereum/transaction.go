package ethereum

import (
	"crypto/ecdsa"
	"math/big"

	"event-pool/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Simplified TX type without From, Hash, or Cache
type txToSign struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *[20]byte // Or your Address type
	Value    *big.Int
	Input    []byte
}

func SignTransaction(tx *types.Transaction, priv *ecdsa.PrivateKey) ([]byte, error) {
	// 1. Prepare tx for signing
	toSign := txToSign{
		Nonce:    tx.Nonce,
		GasPrice: tx.GasPrice,
		Gas:      tx.Gas,
		To:       (*[20]byte)(tx.To), // or convert your Address type
		Value:    tx.Value,
		Input:    tx.Input,
	}

	// 2. RLP encode unsigned tx
	encoded, err := rlp.EncodeToBytes(toSign)
	if err != nil {
		return nil, err
	}

	// 3. Hash it
	hash := crypto.Keccak256Hash(encoded)

	// 4. Sign the hash
	sig, err := crypto.Sign(hash.Bytes(), priv)
	if err != nil {
		return nil, err
	}

	// 5. Parse signature
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	v := uint8(sig[64]) + 27 // Ethereum-style recovery ID

	// 6. Set on tx
	tx.R = r
	tx.S = s
	tx.V = big.NewInt(int64(v))

	// 7. Optional: compute tx hash
	fullTx := &types.Transaction{
		Nonce:    tx.Nonce,
		GasPrice: tx.GasPrice,
		Gas:      tx.Gas,
		To:       tx.To,
		Value:    tx.Value,
		Input:    tx.Input,
		V:        tx.V,
		R:        tx.R,
		S:        tx.S,
	}

	fullEncoded := fullTx.MarshalRLP()

	tx.Hash = types.Hash(crypto.Keccak256Hash(fullEncoded))
	return fullEncoded, nil
}
