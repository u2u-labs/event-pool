package jsonrpc

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"

	"event-pool/helper/tests"
	"event-pool/network/common"
	"github.com/stretchr/testify/assert"
)

func TestHTTPServer(t *testing.T) {
	port, portErr := tests.GetFreePort()

	if portErr != nil {
		t.Fatalf("Unable to fetch free port, %v", portErr)
	}

	config := &Config{
		Store: nil,
		Addr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port},
	}
	_, err := NewJSONRPC(common.NewNullSugaredLogger(), config)

	if err != nil {
		t.Fatal(err)
	}
}

func Test_handleGetRequest(t *testing.T) {
	var (
		chainName = "test"
		chainID   = uint64(200)
	)

	jsonRPC := &JSONRPC{
		config: &Config{
			ChainName: chainName,
			ChainID:   chainID,
		},
	}

	mockWriter := bytes.NewBuffer(nil)

	jsonRPC.handleGetRequest(mockWriter)

	response := &GetResponse{}

	assert.NoError(
		t,
		json.Unmarshal(mockWriter.Bytes(), response),
	)

	assert.Equal(
		t,
		&GetResponse{
			Name:    chainName,
			ChainID: chainID,
			Version: "0.1.0",
		},
		response,
	)
}
