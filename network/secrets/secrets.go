package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (crypto.PrivKey, crypto.PubKey, error) {
	privKey, pubKey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return privKey, pubKey, nil
}

// SavePrivateKey saves a private key to a file
func SavePrivateKey(privKey crypto.PrivKey, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal the private key
	privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Write raw bytes to file
	if err := os.WriteFile(filePath, privKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private key to file: %w", err)
	}

	return nil
}

// LoadPrivateKeyFromPath loads a private key from a file
func LoadPrivateKeyFromPath(filepath string) (crypto.PrivKey, error) {
	// Read file contents
	privKeyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Unmarshal private key directly from raw bytes
	privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	return privKey, nil
}

func LoadPrivateKeyFromString(keyString string) (crypto.PrivKey, error) {
	// Unmarshal private key
	privKey, err := crypto.UnmarshalPrivateKey([]byte(keyString))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	return privKey, nil
}

// SavePublicKey saves a public key to a file
func SavePublicKey(pubKey crypto.PubKey, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal the public key
	pubKeyBytes, err := crypto.MarshalPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	// Write raw bytes to file
	if err := os.WriteFile(filePath, pubKeyBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key to file: %w", err)
	}

	return nil
}

// LoadPublicKey loads a public key from a file
func LoadPublicKey(filepath string) (crypto.PubKey, error) {
	// Read file contents
	pubKeyBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Unmarshal public key directly from raw bytes
	pubKey, err := crypto.UnmarshalPublicKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal public key: %w", err)
	}

	return pubKey, nil
}

// WhitelistManager manages a list of allowed public keys
type WhitelistManager struct {
	allowedPublicKeys map[string]crypto.PubKey
	mu                sync.RWMutex
}

// NewWhitelistManager creates a new WhitelistManager
func NewWhitelistManager() *WhitelistManager {
	return &WhitelistManager{
		allowedPublicKeys: make(map[string]crypto.PubKey),
	}
}

// AddPublicKey adds a public key to the whitelist
func (w *WhitelistManager) AddPublicKey(pubKey crypto.PubKey) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Convert public key to string for storage
	pubKeyBytes, err := pubKey.Raw()
	if err != nil {
		return fmt.Errorf("failed to get public key bytes: %w", err)
	}
	keyStr := base64.StdEncoding.EncodeToString(pubKeyBytes)

	w.allowedPublicKeys[keyStr] = pubKey
	return nil
}

// AddPublicKeyFromFile loads and adds a public key from a file
func (w *WhitelistManager) AddPublicKeyFromFile(filepath string) error {
	pubKey, err := LoadPublicKey(filepath)
	if err != nil {
		return fmt.Errorf("failed to load public key: %w", err)
	}
	return w.AddPublicKey(pubKey)
}

// IsAllowed checks if a public key is in the whitelist
func (w *WhitelistManager) IsAllowed(pubKey crypto.PubKey) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Convert public key to string for comparison
	pubKeyBytes, err := pubKey.Raw()
	if err != nil {
		return false
	}
	keyStr := base64.StdEncoding.EncodeToString(pubKeyBytes)

	_, exists := w.allowedPublicKeys[keyStr]
	return exists
}
