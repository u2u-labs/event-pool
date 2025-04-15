package p2p

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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

	ContractDiscoveryProtocol   = "/contract/discovery/1.0.0"
	ContractQueryTimeout        = time.Second * 5
	NewPeerNotificationProtocol = "/chain/new-peer/1.0.0"
)

type Executor interface {
	ReconstructChain(ctx context.Context, chainID int64)

	// New contract-related methods
	LoadAllChainData(ctx context.Context) error
	RegisterContract(ctx context.Context, chainID int64, contractAddress string, metadata map[string]string) error
	HasContract(ctx context.Context, chainID int64, contractAddress string) bool
	GetContract(ctx context.Context, chainID int64, contractAddress string) (*ContractInfo, bool)
	ValidateContract(ctx context.Context, chainID int64, contractAddress string, responses map[peer.ID]*ContractResponse) (bool, *ContractInfo, error)
}

// ContractInfo represents information about a contract on a specific chain
type ContractInfo struct {
	ChainID         int64
	ContractAddress string
	Metadata        map[string]string
}

// ContractQuery represents a request for contract information
type ContractQuery struct {
	ChainID         int64  `json:"chain_id"`
	ContractAddress string `json:"contract_address"`
}

// ContractResponse represents a response to a contract query
type ContractResponse struct {
	Found    bool              `json:"found"`
	Contract ContractInfo      `json:"contract,omitempty"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
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
	logger := SetupLogger()

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
	h.SetStreamHandler(ContractDiscoveryProtocol, node.handleContractQuery)
	h.SetStreamHandler(NewPeerNotificationProtocol, node.handleNewPeerNotification)

	// Start advertising presence
	node.advertise()

	return node, nil
}

// SetupLogger initializes and returns a zap logger configured for dev or prod
func SetupLogger() *zap.SugaredLogger {
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
	groups, exists := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if !exists {
		// We don't have this chain group
		stream.Write([]byte{0}) // 0 = not found
		return
	}

	// We have this chain group, respond with success
	stream.Write([]byte{1}) // 1 = found

	// get all peers in the group
	groups.peersLock.RLock()
	peers := make([]peer.ID, 0, len(groups.Peers))
	for p := range groups.Peers {
		peers = append(peers, p)
	}
	groups.peersLock.RUnlock()

	// Write existed peers in the group for new peer
	numPeers := len(peers)
	stream.Write([]byte{byte(numPeers)})
	// Write each peer's ID
	for _, p := range peers {
		peerIDBytes := []byte(p)
		stream.Write(peerIDBytes)
	}

	// Add the peer to our chain group
	n.addPeerToChainGroup(chainID, remotePeer)

	// Announce new node join
	n.announceNewPeer(chainID, remotePeer)
}

// Handler for new peer notifications
func (n *ChainNode) handleNewPeerNotification(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()
	n.logger.Infof("Received stream from %s", remotePeer.String())

	reader := bufio.NewReader(stream)

	// Read chain ID with timeout
	chainIDBuf := make([]byte, 8)
	if _, err := io.ReadFull(reader, chainIDBuf); err != nil {
		n.logger.Errorf("Failed to read chain ID: %v", err)
		return
	}
	chainID := int64(binary.BigEndian.Uint64(chainIDBuf))

	// Read peer ID with more robust method
	peerIDLenBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, peerIDLenBuf); err != nil {
		n.logger.Errorf("Failed to read peer ID length: %v", err)
		return
	}
	peerIDLen := binary.BigEndian.Uint16(peerIDLenBuf)

	// Prevent oversized peer ID
	if peerIDLen > 256 {
		n.logger.Errorf("Peer ID length too large: %d", peerIDLen)
		return
	}

	newPeerIDBytes := make([]byte, peerIDLen)
	if _, err := io.ReadFull(reader, newPeerIDBytes); err != nil {
		n.logger.Errorf("Failed to read new peer ID: %v", err)
		return
	}

	newPeerID, err := peer.IDFromBytes(newPeerIDBytes)
	if err != nil {
		n.logger.Errorf("Invalid peer ID: %v", err)
		return
	}

	if err := n.connectToPeer(newPeerID); err != nil {
		n.logger.Errorf("Failed to connect to new peer %s: %v", newPeerID, err)
		stream.Write([]byte{0}) // Negative acknowledgment
		return
	}

	// Add peer to the chain group
	n.addPeerToChainGroup(chainID, newPeerID)

	// Send positive acknowledgment
	stream.Write([]byte{1})
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
					_, err = n.routingDiscovery.Advertise(n.ctx, groupTag)
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
			n.logger.Infow("Found peer in group", "peer", p.ID, "groupTag", groupTag)
		}
	}

	// If we found peers for this chain group
	if len(candidatePeers) > 0 {
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

	// Read existed peers in the group
	numPeersBuf := make([]byte, 1)
	_, err = stream.Read(numPeersBuf)
	if err != nil {
		n.logger.Warnf("Failed to read number of peers: %v", err)
		return err
	}

	numPeers := int(numPeersBuf[0])
	n.logger.Infof("Received %d peers for chain group %d", numPeers, chainID)

	// Create the chain group if it doesn't exist
	n.createNewChainGroup(chainID)

	// Read peer IDs
	for i := 0; i < numPeers; i++ {
		// Read peer ID length (assuming fixed length for simplicity)
		peerIDBytes := make([]byte, len(peerInfo.ID))
		_, err := stream.Read(peerIDBytes)
		if err != nil {
			n.logger.Warnf("Failed to read peer ID: %v", err)
			continue
		}

		// Convert bytes back to peer ID
		peerID, err := peer.IDFromBytes(peerIDBytes)
		if err != nil {
			n.logger.Warnf("Invalid peer ID: %v", err)
			continue
		}

		// Try to connect to the peer
		err = n.connectToPeer(peerID)
		if err != nil {
			n.logger.Warnf("Failed to connect to peer %s: %v", peerID, err)
			continue
		}

		// Add peer to the chain group
		n.addPeerToChainGroup(chainID, peerID)
	}

	// add itself to the chain group
	n.addPeerToChainGroup(chainID, n.host.ID())
	// Reconstruct chain event (placeholder)
	go n.reconstructChain(chainID)

	return nil
}

func (n *ChainNode) connectToPeer(peerID peer.ID) error {
	peerInfo, err := n.dht.FindPeer(n.ctx, peerID)
	if err != nil {
		return fmt.Errorf("could not connect to peer %s through any method", peerID)
	}
	if peerInfo.ID == n.host.ID() {
		return fmt.Errorf("cannot connect to self")
	}

	connectCtx, cancel := context.WithTimeout(n.ctx, ConnectionTimeout)
	defer cancel()

	if err = n.host.Connect(connectCtx, peerInfo); err != nil {
		return err
	}
	n.logger.Infof("Connected to peer %s via DHT", peerID)

	return nil
}

// createNewChainGroup initializes a new chain group in local memory
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
	n.logger.Infow("Added peer to chain group", "host", n.host.ID(), "peer", peerID, "chainId", chainID)
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
	peers := make([]peer.ID, 0, len(group.Peers))
	for peerID := range group.Peers {
		if peerID != newPeerID && peerID != n.host.ID() {
			peers = append(peers, peerID)
		}
	}
	group.peersLock.RUnlock()

	n.logger.Infof("Announcing new peer %s join for chain %d to %d peers", newPeerID, chainID, len(peers))

	// Use a wait group to manage concurrent notifications
	var wg sync.WaitGroup
	notificationChan := make(chan error, len(peers))

	for _, peerID := range peers {
		wg.Add(1)
		go func(targetPeerID peer.ID) {
			defer wg.Done()

			if err := n.connectToPeer(targetPeerID); err != nil {
				notificationChan <- fmt.Errorf("failed to connect to peer %s: %v", targetPeerID, err)
				return
			}

			connectCtx, cancel := context.WithTimeout(n.ctx, ConnectionTimeout)
			defer cancel()
			// Open stream for new peer notification
			stream, err := n.host.NewStream(connectCtx, targetPeerID, NewPeerNotificationProtocol)
			if err != nil {
				notificationChan <- fmt.Errorf("failed to open stream to peer %s: %v", targetPeerID, err)
				return
			}
			defer stream.Close()

			// Prepare notification payload with length-prefixed peer ID
			newPeerIDBytes := []byte(newPeerID)
			payload := make([]byte, 8+2+len(newPeerIDBytes))

			// Write chain ID
			binary.BigEndian.PutUint64(payload[:8], uint64(chainID))

			// Write new peer ID length
			binary.BigEndian.PutUint16(payload[8:10], uint16(len(newPeerIDBytes)))

			// Write new peer ID bytes
			copy(payload[10:], newPeerIDBytes)

			// Send notification
			if _, err := stream.Write(payload); err != nil {
				notificationChan <- fmt.Errorf("failed to send notification to peer %s: %v", targetPeerID, err)
				return
			}

			// Read acknowledgment
			ackBuf := make([]byte, 1)
			if _, err := stream.Read(ackBuf); err != nil {
				notificationChan <- fmt.Errorf("failed to read ack from peer %s: %v", targetPeerID, err)
				return
			}

			if ackBuf[0] != 1 {
				notificationChan <- fmt.Errorf("peer %s did not acknowledge new peer", targetPeerID)
				return
			}

			notificationChan <- nil
		}(peerID)
	}

	// Wait for all notifications to complete
	go func() {
		wg.Wait()
		close(notificationChan)
	}()

	// Handle notification results
	var errors []error
	for err := range notificationChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		n.logger.Warnf("Errors during new peer announcement: %v", errors)
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

// New method for ChainNode
// handleContractQuery delegates contract queries to the Executor
func (n *ChainNode) handleContractQuery(stream network.Stream) {
	defer stream.Close()

	// Read the query
	var query ContractQuery
	err := json.NewDecoder(stream).Decode(&query)
	if err != nil {
		n.logger.Errorf("Error decoding contract query: %v", err)
		return
	}

	n.logger.Infof("Received contract query for chain %d, address %s",
		query.ChainID, query.ContractAddress)

	// Delegate contract lookup to the executor
	exists := n.executor.HasContract(n.ctx, query.ChainID, query.ContractAddress)

	// Create response
	response := ContractResponse{
		Found: exists,
	}

	if exists {
		// If contract exists, get details
		if contractInfo, ok := n.executor.GetContract(n.ctx, query.ChainID, query.ContractAddress); ok {
			response.Contract = *contractInfo
		}
	} else {
		response.Error = "Contract not found"
	}

	// Send response
	err = json.NewEncoder(stream).Encode(response)
	if err != nil {
		n.logger.Errorf("Error encoding contract response: %v", err)
	}
}

// TODO: skeleton implementation for now
// FindContract queries the network for a contract's information
func (n *ChainNode) FindContract(ctx context.Context, chainID int64, contractAddress string) (*ContractInfo, error) {
	// First check if executor has this contract locally
	if n.executor.HasContract(ctx, chainID, contractAddress) {
		contractInfo, _ := n.executor.GetContract(ctx, chainID, contractAddress)
		return contractInfo, nil
	}

	// Get peers for this chain
	n.chainGroupsLock.RLock()
	group, exists := n.chainGroups[chainID]
	n.chainGroupsLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("not connected to chain %d", chainID)
	}

	group.peersLock.RLock()
	peers := make([]peer.ID, 0, len(group.Peers))
	for peerID := range group.Peers {
		if peerID != n.host.ID() { // Exclude self
			peers = append(peers, peerID)
		}
	}
	group.peersLock.RUnlock()

	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers available for chain %d", chainID)
	}

	// Query multiple peers for contract info
	type peerResponse struct {
		peerID   peer.ID
		response *ContractResponse
		err      error
	}

	resultChan := make(chan peerResponse)
	queryCtx, cancel := context.WithTimeout(ctx, ContractQueryTimeout)
	defer cancel()

	// Query each peer
	for _, pID := range peers {
		go func(peerID peer.ID) {
			response, err := n.queryPeerForContract(queryCtx, peerID, chainID, contractAddress)
			resultChan <- peerResponse{peerID, response, err}
		}(pID)
	}

	// Collect responses
	responses := make(map[peer.ID]*ContractResponse)
	var lastError error

	// Wait for responses or timeout
	for i := 0; i < len(peers); i++ {
		select {
		case result := <-resultChan:
			if result.err != nil {
				lastError = result.err
				n.logger.Warnf("Error querying peer %s: %v", result.peerID, result.err)
				continue
			}
			responses[result.peerID] = result.response
		case <-queryCtx.Done():
			if len(responses) == 0 {
				return nil, fmt.Errorf("contract query timed out")
			}
			break
		}
	}

	// Delegate validation to executor to determine consensus and get contract info
	valid, contractInfo, err := n.executor.ValidateContract(ctx, chainID, contractAddress, responses)
	if err != nil {
		if lastError != nil {
			return nil, fmt.Errorf("%v (underlying error: %v)", err, lastError)
		}
		return nil, err
	}

	if !valid {
		return nil, fmt.Errorf("contract not yet indexed or not enough consensus among peers")
	}

	// Register the contract locally if it was found
	if contractInfo != nil {
		n.executor.RegisterContract(ctx, chainID, contractAddress, contractInfo.Metadata)
	}

	return contractInfo, nil
}

// queryPeerForContract sends a contract query to a specific peer
func (n *ChainNode) queryPeerForContract(ctx context.Context, peerID peer.ID,
	chainID int64, contractAddress string) (*ContractResponse, error) {

	// Open stream to peer
	stream, err := n.host.NewStream(ctx, peerID, ContractDiscoveryProtocol)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream to peer %s: %w", peerID, err)
	}
	defer stream.Close()

	// Send query
	query := ContractQuery{
		ChainID:         chainID,
		ContractAddress: contractAddress,
	}

	err = json.NewEncoder(stream).Encode(query)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}

	// Read response
	var response ContractResponse
	err = json.NewDecoder(stream).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
