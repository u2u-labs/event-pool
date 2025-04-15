package p2p

import (
	"sync"

	"event-pool/internal/keyutil"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// WhitelistConnectionGater implements the libp2p ConnectionGater interface
type WhitelistConnectionGater struct {
	whitelist *keyutil.WhitelistManager
	mu        sync.RWMutex
}

// Interface compliance check
var _ connmgr.ConnectionGater = (*WhitelistConnectionGater)(nil)

// InterceptPeerDial determines if a peer can be dialed
func (w *WhitelistConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If no whitelist is set, allow all connections
	if w.whitelist == nil {
		return true
	}

	// Get the public key for the peer
	return w.checkPeerAllowed(p)
}

// InterceptAddrDial determines if a specific address can be dialed
func (w *WhitelistConnectionGater) InterceptAddrDial(p peer.ID, addr ma.Multiaddr) (allow bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If no whitelist is set, allow all connections
	if w.whitelist == nil {
		return true
	}

	// Check if the peer is allowed
	return w.checkPeerAllowed(p)
}

// InterceptAcceptDial determines if an incoming connection should be accepted
func (w *WhitelistConnectionGater) InterceptAcceptDial(p peer.ID) (allow bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If no whitelist is set, allow all connections
	if w.whitelist == nil {
		return true
	}

	// Check if the peer is allowed
	return w.checkPeerAllowed(p)
}

// Helper method to check if a peer is allowed
func (w *WhitelistConnectionGater) checkPeerAllowed(p peer.ID) bool {
	// Retrieve the public key for the peer
	pubKey, err := p.ExtractPublicKey()
	if err != nil {
		// If we can't extract the public key, deny the connection
		return false
	}

	// Check if the public key is in the whitelist
	return w.whitelist.IsAllowed(pubKey)
}

// Implement other required ConnectionGater methods (these can be pass-through)
func (w *WhitelistConnectionGater) InterceptSecured(dir network.Direction, p peer.ID, addr network.ConnMultiaddrs) (allow bool) {
	return true
}

func (w *WhitelistConnectionGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}

// InterceptAccept determines if an incoming connection should be accepted
func (w *WhitelistConnectionGater) InterceptAccept(network.ConnMultiaddrs) (allow bool) {
	// Always allow incoming connection attempts
	// Actual peer validation will happen in later stages
	return true
}
