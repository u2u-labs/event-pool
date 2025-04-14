package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ChainProtocol         = "/chain/1.0.0"
	ChainDiscoveryTag     = "chain-discovery"
	ChainGroupPrefix      = "chain-group-"
	DiscoveryInterval     = time.Minute * 10
	ConnectionTimeout     = time.Second * 10
	MaxConcurrentRequests = 10
)

type Executor interface {
	ReconstructChain(ctx context.Context, chainID int64)
}

// ChainNode represents a node in the p2p network capable of joining multiple chain groups
type ChainNode struct {
	host             host.Host
	dht              *dht.IpfsDHT
	routingDiscovery *drouting.RoutingDiscovery
	logger           *zap.SugaredLogger
	ctx              context.Context

	// Chain group management
	chainGroups     map[int64]*ChainGroup
	chainGroupsLock sync.RWMutex
	executor        Executor
}

// ChainGroup represents a group of nodes working on a specific chain ID
type ChainGroup struct {
	ChainID   int64
	Peers     map[peer.ID]struct{}
	peersLock sync.RWMutex
}

// NewChainNode creates and initializes a new chain node
func NewChainNode(ctx context.Context, bootstrapPeers []multiaddr.Multiaddr, e Executor) (*ChainNode, error) {
	// Setup logger
	logger := setupLogger()

	// Create libp2p node
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.EnableNATService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	logger.Infof("Host created. ID: %s", h.ID().String())
	logger.Infof("Listening on:")
	for _, addr := range h.Addrs() {
		logger.Infof("  %s/p2p/%s", addr.String(), h.ID().String())
	}

	// Setup DHT
	idht, err := dht.New(ctx, h, dht.Mode(dht.ModeAutoServer))
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	// Connect to bootstrap peers
	if len(bootstrapPeers) > 0 {
		logger.Info("Connecting to bootstrap peers...")

		var wg sync.WaitGroup
		connChan := make(chan struct{}, MaxConcurrentRequests)

		for _, peerAddr := range bootstrapPeers {
			wg.Add(1)
			go func(pAddr multiaddr.Multiaddr) {
				defer wg.Done()
				connChan <- struct{}{}
				defer func() { <-connChan }()

				peerInfo, err := peer.AddrInfoFromP2pAddr(pAddr)
				if err != nil {
					logger.Warnf("Failed to parse bootstrap peer address: %v", err)
					return
				}

				connectCtx, cancel := context.WithTimeout(ctx, ConnectionTimeout)
				defer cancel()

				if err := h.Connect(connectCtx, *peerInfo); err != nil {
					logger.Warnf("Failed to connect to bootstrap peer %s: %v", pAddr, err)
					return
				}
				logger.Infof("Connected to bootstrap peer: %s", pAddr)
			}(peerAddr)
		}
		wg.Wait()
	}

	// Initialize the DHT
	logger.Info("Bootstrapping DHT...")
	if err := idht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Create routing discovery
	routingDiscovery := drouting.NewRoutingDiscovery(idht)

	// Setup local peer discovery using mDNS
	setupMDNSDiscovery(ctx, h, logger)

	node := &ChainNode{
		host:             h,
		dht:              idht,
		routingDiscovery: routingDiscovery,
		logger:           logger,
		ctx:              ctx,
		chainGroups:      make(map[int64]*ChainGroup),
		executor:         e,
	}

	// Set protocol handler
	h.SetStreamHandler(ChainProtocol, node.handleStream)

	// Start advertising presence
	node.advertise()

	return node, nil
}

// setupLogger initializes and returns a zap logger configured for dev or prod
func setupLogger() *zap.SugaredLogger {
	var cfg zap.Config

	// Check environment and configure logger accordingly
	// This is a placeholder - replace with your environment detection logic
	isDev := true // Example condition - set based on your requirements
	mode := os.Getenv("MODE")
	if mode == "prod" {
		isDev = false
	}

	if isDev {
		// Development configuration
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		// Production configuration
		cfg = zap.NewProductionConfig()
		cfg.DisableCaller = true
	}

	logger, err := cfg.Build()
	if err != nil {
		// Fallback to a simple logger if the configured one fails
		fallback := zap.NewExample().Sugar()
		fallback.Warnf("Failed to create configured logger: %v", err)
		return fallback
	}

	return logger.Sugar()
}

// handleStream processes incoming streams from other peers
func (n *ChainNode) handleStream(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()
	n.logger.Infof("Received stream from %s", remotePeer.String())

	// Read the request
	buf := make([]byte, 8) // For chain ID (int64)
	_, err := stream.Read(buf)
	if err != nil {
		n.logger.Errorf("Error reading stream from %s: %v", remotePeer, err)
		return
	}

	chainID := int64(binary.BigEndian.Uint64(buf))
	n.logger.Infof("Peer %s is requesting information for chain ID: %d", remotePeer, chainID)

	// Check if we have this chain group
	n.chainGroupsLock.RLock()
	_, exists := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if !exists {
		// We don't have this chain group
		stream.Write([]byte{0}) // 0 = not found
		return
	}

	// We have this chain group, respond with success
	stream.Write([]byte{1}) // 1 = found

	// Add the peer to our chain group
	n.addPeerToChainGroup(chainID, remotePeer)

	// Announce new node join
	n.announceNewPeer(chainID, remotePeer)
}

// setupMDNSDiscovery configures mDNS for local peer discovery
func setupMDNSDiscovery(ctx context.Context, h host.Host, logger *zap.SugaredLogger) {
	// Setup local discovery using mDNS
	mdnsService := mdns.NewMdnsService(h, ChainDiscoveryTag, &mdnsNotifee{h: h, logger: logger})
	if err := mdnsService.Start(); err != nil {
		logger.Warnf("Error starting mDNS service: %v", err)
	}
}

// mdnsNotifee handles mDNS peer discovery notifications
type mdnsNotifee struct {
	h      host.Host
	logger *zap.SugaredLogger
}

// HandlePeerFound is called when mDNS discovers a new peer
func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Infof("Discovered new peer via mDNS: %s", pi.ID)
	if pi.ID == n.h.ID() {
		// Skip self
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ConnectionTimeout)
	defer cancel()

	if err := n.h.Connect(ctx, pi); err != nil {
		n.logger.Warnf("Failed to connect to mDNS peer %s: %v", pi.ID, err)
	}
}

// advertise publishes node's presence on the DHT
func (n *ChainNode) advertise() {
	ticker := time.NewTicker(DiscoveryInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Advertise this node
				_, err := n.routingDiscovery.Advertise(n.ctx, ChainDiscoveryTag)
				if err != nil {
					n.logger.Warnf("Error advertising self: %v", err)
				}

				// Also advertise for each chain group
				n.chainGroupsLock.RLock()
				for chainID := range n.chainGroups {
					groupTag := fmt.Sprintf("%s%d", ChainGroupPrefix, chainID)
					_, err := n.routingDiscovery.Advertise(n.ctx, groupTag)
					if err != nil {
						n.logger.Warnf("Error advertising chain group %d: %v", chainID, err)
					}
				}
				n.chainGroupsLock.RUnlock()
			case <-n.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// JoinChainGroup attempts to join a chain group with the specified chainID
func (n *ChainNode) JoinChainGroup(chainID int64) error {
	// Check if we're already in this chain group
	n.chainGroupsLock.RLock()
	_, alreadyJoined := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if alreadyJoined {
		n.logger.Infof("Already joined chain group %d", chainID)
		return nil
	}

	// Look for peers in this chain group
	groupTag := fmt.Sprintf("%s%d", ChainGroupPrefix, chainID)
	n.logger.Infof("Looking for peers in chain group %d...", chainID)

	peers, err := n.routingDiscovery.FindPeers(n.ctx, groupTag)
	if err != nil {
		return fmt.Errorf("error finding peers for chain %d: %w", chainID, err)
	}

	// Filter out self and peers we're already connected to
	candidatePeers := make([]peer.AddrInfo, 0)
	for p := range peers {
		if p.ID != n.host.ID() {
			candidatePeers = append(candidatePeers, p)
		}
		n.logger.Infow("Found peer", "peer", p.ID)
	}

	// If we found peers for this chain group
	if len(candidatePeers) > 0 {
		n.logger.Infof("Found %d potential peers for chain %d", len(candidatePeers), chainID)

		// Try to join the chain group via one of the peers
		for _, peerInfo := range candidatePeers {
			if err := n.joinViaExistingPeer(peerInfo, chainID); err == nil {
				n.logger.Infof("Successfully joined chain group %d via peer %s", chainID, peerInfo.ID)
				// Successfully joined group
				return nil
			}
		}
	}

	// If we couldn't join via existing peers, create a new group
	n.logger.Infof("Creating new chain group %d", chainID)
	n.createNewChainGroup(chainID)
	n.addPeerToChainGroup(chainID, n.host.ID())

	// Advertise the new group
	groupTag = fmt.Sprintf("%s%d", ChainGroupPrefix, chainID)
	_, err = n.routingDiscovery.Advertise(n.ctx, groupTag)
	if err != nil {
		n.logger.Warnf("Error advertising new chain group %d: %v", chainID, err)
	}

	return nil
}

func (n *ChainNode) JoinChainGroups(chainIDs ...int64) error {
	for _, chainID := range chainIDs {
		if err := n.JoinChainGroup(chainID); err != nil {
			return err
		}
	}
	return nil
}

// joinViaExistingPeer tries to join a chain group through an existing peer
func (n *ChainNode) joinViaExistingPeer(peerInfo peer.AddrInfo, chainID int64) error {
	// Connect to the peer if not already connected
	if n.host.Network().Connectedness(peerInfo.ID) != network.Connected {
		ctx, cancel := context.WithTimeout(n.ctx, ConnectionTimeout)
		defer cancel()

		n.logger.Infof("Connecting to peer %s for chain %d", peerInfo.ID, chainID)
		if err := n.host.Connect(ctx, peerInfo); err != nil {
			n.logger.Warnf("Failed to connect to peer %s: %v", peerInfo.ID, err)
			return err
		}
	}

	// Open a stream to the peer
	ctx, cancel := context.WithTimeout(n.ctx, ConnectionTimeout)
	defer cancel()

	n.logger.Infof("Opening stream to peer %s for chain %d", peerInfo.ID, chainID)
	stream, err := n.host.NewStream(ctx, peerInfo.ID, ChainProtocol)
	if err != nil {
		n.logger.Warnf("Failed to open stream to peer %s: %v", peerInfo.ID, err)
		return err
	}
	defer stream.Close()

	// Send chain ID
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(chainID))
	_, err = stream.Write(buf)
	if err != nil {
		n.logger.Warnf("Failed to write to stream: %v", err)
		return err
	}

	// Read response
	responseBuf := make([]byte, 1)
	_, err = stream.Read(responseBuf)
	if err != nil {
		n.logger.Warnf("Failed to read from stream: %v", err)
		return err
	}

	if responseBuf[0] == 0 {
		// Peer doesn't have this chain group
		n.logger.Warnf("Peer %s doesn't have chain group %d", peerInfo.ID, chainID)
		return fmt.Errorf("peer doesn't have chain group")
	}

	// Successfully joined the chain group
	n.createNewChainGroup(chainID)
	n.addPeerToChainGroup(chainID, peerInfo.ID)

	// Reconstruct chain event (placeholder)
	go n.reconstructChain(chainID)

	return nil
}

// createNewChainGroup initializes a new chain group
func (n *ChainNode) createNewChainGroup(chainID int64) {
	n.chainGroupsLock.Lock()
	defer n.chainGroupsLock.Unlock()

	// Check if we already have this group (double-check)
	if _, exists := n.chainGroups[chainID]; !exists {
		n.chainGroups[chainID] = &ChainGroup{
			ChainID: chainID,
			Peers:   make(map[peer.ID]struct{}),
		}
		n.logger.Infof("Created new chain group for chain ID %d", chainID)
	}
}

// addPeerToChainGroup adds a peer to the specified chain group
func (n *ChainNode) addPeerToChainGroup(chainID int64, peerID peer.ID) {
	n.chainGroupsLock.Lock()
	defer n.chainGroupsLock.Unlock()

	group, exists := n.chainGroups[chainID]
	if !exists {
		group = &ChainGroup{
			ChainID: chainID,
			Peers:   make(map[peer.ID]struct{}),
		}
		n.chainGroups[chainID] = group
	}

	group.peersLock.Lock()
	defer group.peersLock.Unlock()

	group.Peers[peerID] = struct{}{}
	n.logger.Infof("Added peer %s to chain group %d", peerID, chainID)
}

// announceNewPeer broadcasts to all peers in a chain group that a new peer has joined
func (n *ChainNode) announceNewPeer(chainID int64, newPeerID peer.ID) {
	n.chainGroupsLock.RLock()
	group, exists := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if !exists {
		n.logger.Warnf("Cannot announce for non-existent chain group %d", chainID)
		return
	}

	group.peersLock.RLock()
	defer group.peersLock.RUnlock()

	n.logger.Infof("Announcing new peer %s join for chain %d", newPeerID, chainID)

	// This would typically involve sending messages to peers
	// For now, we'll just log it
	for peerID := range group.Peers {
		if peerID != newPeerID && peerID != n.host.ID() {
			n.logger.Infof("Notifying peer %s about new peer %s in chain %d", peerID, newPeerID, chainID)
			// Here you would implement actual notification logic
		}
	}
}

// reconstructChain handles the reconstruction of chain data after joining a group
func (n *ChainNode) reconstructChain(chainID int64) {
	n.logger.Infof("Starting chain reconstruction for chain ID %d", chainID)
	defer n.logger.Infof("Chain reconstruction complete for chain ID %d", chainID)
	// TODO: Implement chain reconstruction logic
}

// GetChainPeers returns the list of peers in a specific chain group
func (n *ChainNode) GetChainPeers(chainID int64) []peer.ID {
	n.chainGroupsLock.RLock()
	group, exists := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if !exists {
		return nil
	}

	group.peersLock.RLock()
	defer group.peersLock.RUnlock()

	peers := make([]peer.ID, 0, len(group.Peers))
	for peerID := range group.Peers {
		peers = append(peers, peerID)
	}

	return peers
}

// ListChainGroups returns all chain groups this node is part of
func (n *ChainNode) ListChainGroups() []int64 {
	n.chainGroupsLock.RLock()
	defer n.chainGroupsLock.RUnlock()

	groups := make([]int64, 0, len(n.chainGroups))
	for chainID := range n.chainGroups {
		groups = append(groups, chainID)
	}

	return groups
}

// Close shuts down the node
func (n *ChainNode) Close() error {
	var errs []error

	// Close DHT
	if err := n.dht.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing DHT: %w", err))
	}

	// Close host
	if err := n.host.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing host: %w", err))
	}

	// Log errors if any
	if len(errs) > 0 {
		for _, err := range errs {
			n.logger.Errorf("Close error: %v", err)
		}
		return fmt.Errorf("errors occurred during shutdown: %d errors", len(errs))
	}

	n.logger.Info("Node shutdown complete")
	return nil
}
