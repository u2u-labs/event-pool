package types

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

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
		  "chainId": 1,
		  "stateRoot": "0xab00000000000000000000000000000000000000000000000000000000000000",
		  "creator": "0x1800000000000000000000000000000000000000",
		  "number": 1,
		  "timestamp": 199920,
		  "extraData": "Cg=="
		}`
		header = Header{
			Hash:       Hash{0x8},
			ParentHash: Hash{0x1},
			Creator:    Address{0x18},
			StateRoot:  Hash{0xab},
			ChainId:    1,
			Number:     1,
			Timestamp:  199920,
			ExtraData:  []byte{0xa},
		}
		rg = regexp.MustCompile(`(\t|\n| )+`)
	)

	t.Run("Header marshalled to JSON", func(t *testing.T) {
		t.Parallel()

		marshalledHeader, err := json.Marshal(&header)
		if err != nil {
			t.Fatalf("Unable to marshal header,  %v", err)
		}

		fmt.Println(string(marshalledHeader))
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
