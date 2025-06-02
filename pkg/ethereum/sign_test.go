package ethereum

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	crypto2 "event-pool/crypto"
	"event-pool/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestSignTx(t *testing.T) {
	filter := types.FilterLogsParams{
		FromBlock:       big.NewInt(10000),
		ToBlock:         big.NewInt(10005),
		ContractAddress: common.Address{0x1},
		EventSignature:  common.Hash{0xab},
		ChainId:         5,
	}
	input, err := json.Marshal(filter)
	if err != nil {
		panic(err)
	}

	addr := types.StringToAddress("0x01857E2BCFcb8B4eF76Df6590F8dCd3bf736C9E9")
	tx := &types.Transaction{
		Nonce:    0,
		GasPrice: big.NewInt(1000000000),
		Gas:      21000,
		To:       &addr,
		Value:    big.NewInt(1000000000000000000),
		Input:    input,
	}

	secretBytes, err := os.ReadFile("../../data/temp/consensus/validator.key")
	if err != nil {
		panic(err)
	}
	priv, err := crypto2.BytesToECDSAPrivateKey(secretBytes)
	if err != nil {
		panic(err)
	}

	rawBytes, err := SignTransaction(tx, priv)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Raw bytes: %x\n", rawBytes)
}
