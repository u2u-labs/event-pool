package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func GetTwoNodesSetup(t *testing.T, ctx context.Context, e1 Executor, e2 Executor) (*ChainNode, *ChainNode) {
	// Create first node that will host the group
	node1, err := NewChainNode(ctx, nil, e1)
	require.NoError(t, err, "Failed to create first node")

	// Create a chain group in the first node
	chainID := int64(1)
	err = node1.JoinChainGroup(chainID)
	require.NoError(t, err, "Failed to create chain group in first node")

	// Get node1's address info to use as bootstrap for node2
	node1AddrInfo := peer.AddrInfo{
		ID:    node1.host.ID(),
		Addrs: node1.host.Addrs(),
	}

	// Convert node1's info to multiaddr for bootstrap
	node1Multiaddrs, err := peer.AddrInfoToP2pAddrs(&node1AddrInfo)
	require.NoError(t, err, "Failed to convert AddrInfo to multiaddr")

	// Create second node with first node as bootstrap
	node2, err := NewChainNode(ctx, node1Multiaddrs, e2)
	require.NoError(t, err, "Failed to create second node")

	// Wait a moment for discovery
	time.Sleep(1 * time.Second)

	// Join the same chain group
	err = node2.JoinChainGroup(chainID)
	require.NoError(t, err, "Failed to join existing chain group")

	// Wait for group synchronization
	time.Sleep(1 * time.Second)

	// Verify both nodes list the chain group
	assert.Contains(t, node1.ListChainGroups(), chainID, "First node should be in chain group %d", chainID)
	assert.Contains(t, node2.ListChainGroups(), chainID, "Second node should be in chain group %d", chainID)

	return node1, node2
}
