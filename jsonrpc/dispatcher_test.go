package jsonrpc

import (
	"encoding/json"
	"math/big"
)

var (
	oneEther = new(big.Int).Mul(
		big.NewInt(1),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
)

func toArgUint64Ptr(value uint64) *argUint64 {
	argValue := argUint64(value)

	return &argValue
}

func toArgBytesPtr(value []byte) *argBytes {
	argValue := argBytes(value)

	return &argValue
}

func expectJSONResult(data []byte, v interface{}) error {
	var resp SuccessResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	if err := json.Unmarshal(resp.Result, v); err != nil {
		return err
	}

	return nil
}
