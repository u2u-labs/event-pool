package types

import (
	"encoding/json"
	"math/big"
	"regexp"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

// TestHeader_JSON makes sure the Header is properly
// marshalled and unmarshalled from JSON
func TestHeader_JSON(t *testing.T) {
	t.Parallel()

	var (
		headerJSON = `{
		  "hash": "0x0800000000000000000000000000000000000000000000000000000000000000",
		  "parentHash": "0x0100000000000000000000000000000000000000000000000000000000000000",
		  "logs": [
			{
			  "address": "0x0100000000000000000000000000000000000000",
			  "topics": [
				"0x1000000000000000000000000000000000000000000000000000000000000000",
				"0x1100000000000000000000000000000000000000000000000000000000000000"
			  ],
			  "data": "0x",
			  "blockNumber": "0x1",
			  "transactionHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			  "transactionIndex": "0x0",
			  "blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			  "logIndex": "0x0",
			  "removed": false
			}
		  ],
		  "filter": {
			"BlockHash": null,
			"FromBlock": 1,
			"ToBlock": 2,
			"Addresses": [
			  "0x0100000000000000000000000000000000000000"
			],
			"Topics": [
			  [
				"0x1000000000000000000000000000000000000000000000000000000000000000",
				"0x1100000000000000000000000000000000000000000000000000000000000000"
			  ]
			]
		  }
		}`
		header = Header{
			Hash:       Hash{0x8},
			ParentHash: Hash{0x1},
			Logs: []*types.Log{
				{
					Address:     common.Address{0x1},
					Topics:      []common.Hash{{0x10}, {0x11}},
					Data:        []byte{},
					BlockNumber: 1,
					TxHash:      common.Hash{},
					TxIndex:     0,
					BlockHash:   common.Hash{},
					Index:       0,
					Removed:     false,
				},
			},
			Filter: ethereum.FilterQuery{
				BlockHash: nil,
				FromBlock: big.NewInt(1),
				ToBlock:   big.NewInt(2),
				Addresses: []common.Address{{0x1}},
				Topics:    [][]common.Hash{{common.Hash{0x10}, common.Hash{0x11}}},
			},
		}
		rg = regexp.MustCompile(`(\t|\n| )+`)
	)

	t.Run("Header marshalled to JSON", func(t *testing.T) {
		t.Parallel()

		marshalledHeader, err := json.Marshal(&header)
		if err != nil {
			t.Fatalf("Unable to marshal header,  %v", err)
		}

		assert.Equal(t, rg.ReplaceAllString(headerJSON, ""), string(marshalledHeader))
	})

	t.Run("Header unmarshalled from JSON", func(t *testing.T) {
		t.Parallel()

		unmarshalledHeader := Header{}
		if err := json.Unmarshal([]byte(headerJSON), &unmarshalledHeader); err != nil {
			t.Fatalf("unable to unmarshall JSON, %v", err)
		}

		assert.Equal(t, header, unmarshalledHeader)
	})
}
