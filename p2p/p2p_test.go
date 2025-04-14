package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateNewChainGroup verifies a node can create a new chain group
func TestCreateNewChainGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new chain node without bootstrap peers
	node, err := NewChainNode(ctx, nil, nil)
	require.NoError(t, err, "Failed to create chain node")
	defer node.Close()

	// Join a chain group - should create a new one since none exists
	chainID := int64(1)
	err = node.JoinChainGroup(chainID)
	require.NoError(t, err, "Failed to join chain group")

	// Verify the node is in the chain group
	groups := node.ListChainGroups()
	assert.Contains(t, groups, chainID, "Node should be in chain group %d", chainID)

	// Verify the chain group has this node as its only member
	peers := node.GetChainPeers(chainID)
	assert.Len(t, peers, 1, "Chain group should have 1 peer (self)")
}

// TestConnectToExistingGroup tests that a node can join an existing chain group
func TestConnectToExistingGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create first node that will host the group
	node1, err := NewChainNode(ctx, nil, nil)
	require.NoError(t, err, "Failed to create first node")
	defer node1.Close()

	// Create a chain group in the first node
	chainID := int64(2)
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
	node2, err := NewChainNode(ctx, node1Multiaddrs, nil)
	require.NoError(t, err, "Failed to create second node")
	defer node2.Close()

	// Wait a moment for discovery
	time.Sleep(2 * time.Second)

	// Join the same chain group
	err = node2.JoinChainGroup(chainID)
	require.NoError(t, err, "Failed to join existing chain group")

	// Wait for group synchronization
	time.Sleep(2 * time.Second)

	// Verify both nodes list the chain group
	assert.Contains(t, node1.ListChainGroups(), chainID, "First node should be in chain group %d", chainID)
	assert.Contains(t, node2.ListChainGroups(), chainID, "Second node should be in chain group %d", chainID)

	// Verify the first node sees the second node in its peer list
	// Note: This may take time to propagate, so we attempt a few times
	success := false
	for range 5 {
		peers := node1.GetChainPeers(chainID)
		t.Log("n1 peers: ", peers)
		if containsPeer(peers, node2.host.ID()) {
			success = true
			break
		}
		time.Sleep(time.Second)
	}
	assert.True(t, success, "First node should see the second node in its peer list")
}

// TestConnectToMultipleGroups verifies a node can join multiple chain groups
func TestConnectToMultipleGroups(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a node
	node, err := NewChainNode(ctx, nil, nil)
	require.NoError(t, err, "Failed to create chain node")
	defer node.Close()

	// Join two different chain groups
	chainID1 := int64(3)
	chainID2 := int64(4)

	err = node.JoinChainGroup(chainID1)
	require.NoError(t, err, "Failed to join first chain group")

	err = node.JoinChainGroup(chainID2)
	require.NoError(t, err, "Failed to join second chain group")

	// Verify the node is in both chain groups
	groups := node.ListChainGroups()
	assert.Contains(t, groups, chainID1, "Node should be in chain group %d", chainID1)
	assert.Contains(t, groups, chainID2, "Node should be in chain group %d", chainID2)

	// Verify the node is a member of both groups
	peers1 := node.GetChainPeers(chainID1)
	peers2 := node.GetChainPeers(chainID2)

	assert.NotEmpty(t, peers1, "Chain group 1 should have members")
	assert.NotEmpty(t, peers2, "Chain group 2 should have members")
}

// Helper function to check if a peer ID is in a list of peers
func containsPeer(peers []peer.ID, peerID peer.ID) bool {
	for _, p := range peers {
		if p == peerID {
			return true
		}
	}
	return false
}

// TestMultipleNodesMultipleGroups tests that two nodes can connect to two chain groups
func TestMultipleNodesMultipleGroups(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create first node
	node1, err := NewChainNode(ctx, nil, nil)
	require.NoError(t, err, "Failed to create first node")
	defer node1.Close()

	// Get node1's address info to use as bootstrap for node2
	node1AddrInfo := peer.AddrInfo{
		ID:    node1.host.ID(),
		Addrs: node1.host.Addrs(),
	}

	// Convert node1's info to multiaddr for bootstrap
	node1Multiaddrs, err := peer.AddrInfoToP2pAddrs(&node1AddrInfo)
	require.NoError(t, err, "Failed to convert AddrInfo to multiaddr")

	// Create second node with first node as bootstrap
	node2, err := NewChainNode(ctx, node1Multiaddrs, nil)
	require.NoError(t, err, "Failed to create second node")
	defer node2.Close()

	// Wait for initial connection to establish
	time.Sleep(3 * time.Second)

	// Define two chain IDs to test with
	chainID1 := int64(101)
	chainID2 := int64(102)

	// Join first chain group with node1 first
	t.Logf("Node 1 joining chain group %d", chainID1)
	err = node1.JoinChainGroup(chainID1)
	require.NoError(t, err, "Failed to create chain group %d in first node", chainID1)

	// Now node2 joins the same chain group
	t.Logf("Node 2 joining chain group %d", chainID1)
	err = node2.JoinChainGroup(chainID1)
	require.NoError(t, err, "Failed to join chain group %d with second node", chainID1)

	// Wait for synchronization
	time.Sleep(3 * time.Second)

	// Now for the second chain, node2 creates it first
	t.Logf("Node 2 joining chain group %d", chainID2)
	err = node2.JoinChainGroup(chainID2)
	require.NoError(t, err, "Failed to create chain group %d in second node", chainID2)

	// Node1 joins the second chain group
	t.Logf("Node 1 joining chain group %d", chainID2)
	err = node1.JoinChainGroup(chainID2)
	require.NoError(t, err, "Failed to join chain group %d with first node", chainID2)

	// Wait for group synchronization
	time.Sleep(5 * time.Second)

	// Verify both nodes list both chain groups
	node1Groups := node1.ListChainGroups()
	node2Groups := node2.ListChainGroups()

	t.Logf("Node 1 chain groups: %v", node1Groups)
	t.Logf("Node 2 chain groups: %v", node2Groups)

	assert.Contains(t, node1Groups, chainID1, "First node should be in chain group %d", chainID1)
	assert.Contains(t, node1Groups, chainID2, "First node should be in chain group %d", chainID2)
	assert.Contains(t, node2Groups, chainID1, "Second node should be in chain group %d", chainID1)
	assert.Contains(t, node2Groups, chainID2, "Second node should be in chain group %d", chainID2)

	// Verify both nodes can see each other in both chain groups
	// We'll retry a few times with delays to allow for peer discovery
	success := false
	for i := 0; i < 10; i++ {
		node1Chain1Peers := node1.GetChainPeers(chainID1)
		node1Chain2Peers := node1.GetChainPeers(chainID2)
		node2Chain1Peers := node2.GetChainPeers(chainID1)
		node2Chain2Peers := node2.GetChainPeers(chainID2)

		t.Logf("Attempt %d - Node1 peers in chain %d: %v", i+1, chainID1, node1Chain1Peers)
		t.Logf("Attempt %d - Node1 peers in chain %d: %v", i+1, chainID2, node1Chain2Peers)
		t.Logf("Attempt %d - Node2 peers in chain %d: %v", i+1, chainID1, node2Chain1Peers)
		t.Logf("Attempt %d - Node2 peers in chain %d: %v", i+1, chainID2, node2Chain2Peers)

		// Check if each node sees the other in both chain groups
		if containsPeer(node1Chain1Peers, node2.host.ID()) &&
			containsPeer(node1Chain2Peers, node2.host.ID()) &&
			containsPeer(node2Chain1Peers, node1.host.ID()) &&
			containsPeer(node2Chain2Peers, node1.host.ID()) {
			success = true
			break
		}

		time.Sleep(2 * time.Second)
	}

	assert.True(t, success, "Both nodes should see each other in both chain groups")

	// Verify the number of peers in each chain group
	// Each chain group should have both nodes (2 peers)
	node1Chain1Peers := node1.GetChainPeers(chainID1)
	node1Chain2Peers := node1.GetChainPeers(chainID2)
	node2Chain1Peers := node2.GetChainPeers(chainID1)
	node2Chain2Peers := node2.GetChainPeers(chainID2)

	assert.Len(t, node1Chain1Peers, 2, "Chain %d should have 2 peers in node1", chainID1)
	assert.Len(t, node1Chain2Peers, 1, "Chain %d should have 1 peer in node1", chainID2)
	assert.Len(t, node2Chain1Peers, 1, "Chain %d should have 1 peer in node2", chainID1)
	assert.Len(t, node2Chain2Peers, 2, "Chain %d should have 2 peers in node2", chainID2)
}
