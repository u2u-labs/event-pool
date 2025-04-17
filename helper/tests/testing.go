package tests

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"event-pool/crypto"
	"event-pool/types"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

var (
	ErrTimeout = errors.New("timeout")
)

func GenerateKeyAndAddr(t *testing.T) (*ecdsa.PrivateKey, types.Address) {
	t.Helper()

	key, err := crypto.GenerateECDSAKey()

	assert.NoError(t, err)

	addr := crypto.PubKeyToAddress(&key.PublicKey)

	return key, addr
}

func GenerateTestMultiAddr(t *testing.T) multiaddr.Multiaddr {
	t.Helper()

	priv, _, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Secp256k1, 256)
	if err != nil {
		t.Fatalf("Unable to generate key pair, %v", err)
	}

	nodeID, err := peer.IDFromPrivateKey(priv)
	assert.NoError(t, err)

	port, portErr := GetFreePort()
	if portErr != nil {
		t.Fatalf("Unable to fetch free port, %v", portErr)
	}

	addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, nodeID))
	assert.NoError(t, err)

	return addr
}

func RetryUntilTimeout(ctx context.Context, f func() (interface{}, bool)) (interface{}, error) {
	type result struct {
		data interface{}
		err  error
	}

	resCh := make(chan result, 1)

	go func() {
		defer close(resCh)

		for {
			select {
			case <-ctx.Done():
				resCh <- result{nil, ErrTimeout}

				return
			default:
				res, retry := f()
				if !retry {
					resCh <- result{res, nil}

					return
				}
			}
			time.Sleep(time.Second)
		}
	}()

	res := <-resCh

	return res.data, res.err
}

// GetFreePort asks the kernel for a free open port that is ready to use
func GetFreePort() (port int, err error) {
	var addr *net.TCPAddr

	if addr, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener

		if l, err = net.ListenTCP("tcp", addr); err == nil {
			defer func(l *net.TCPListener) {
				_ = l.Close()
			}(l)

			netAddr, ok := l.Addr().(*net.TCPAddr)
			if !ok {
				return 0, errors.New("invalid type assert to TCPAddr")
			}

			return netAddr.Port, nil
		}
	}

	return
}
