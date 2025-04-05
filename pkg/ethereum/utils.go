package ethereum

import (
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// HexToAddress converts a hex string to an Ethereum address
func HexToAddress(hex string) common.Address {
	// Ensure the hex string has the 0x prefix
	if !strings.HasPrefix(hex, "0x") {
		hex = "0x" + hex
	}
	return common.HexToAddress(hex)
}

// HexToHash converts a hex string to an Ethereum hash
func HexToHash(hex string) common.Hash {
	// Ensure the hex string has the 0x prefix
	if !strings.HasPrefix(hex, "0x") {
		hex = "0x" + hex
	}
	return common.HexToHash(hex)
}

// IsValidAddress checks if a string is a valid Ethereum address
func IsValidAddress(address string) bool {
	// Ensure the address has the 0x prefix
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	return common.IsHexAddress(address)
}

// Log is an alias for types.Log to make it available in this package
type Log = types.Log

// ParseEventSignature converts a readable event signature to the proper format
// Example: "PairCreated(indexed address,indexed address,address,uint256)" -> "0x7e644d79422f17a01f489e7fce88d0ca248c4b3e72a9295cfd1fa6234ac704b4"
// Also handles: "TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)"
func ParseEventSignature(readableSig string) (string, error) {
	log.Printf("Parsing event signature: %s", readableSig)

	// Remove any whitespace
	readableSig = strings.ReplaceAll(readableSig, " ", "")
	log.Printf("After removing whitespace: %s", readableSig)

	// Check if the format is valid
	if !strings.Contains(readableSig, "(") || !strings.HasSuffix(readableSig, ")") {
		log.Printf("Invalid event signature format: missing parentheses")
		return "", fmt.Errorf("invalid event signature format")
	}

	// Extract the event name and parameters
	parts := strings.SplitN(readableSig, "(", 2)
	if len(parts) != 2 {
		log.Printf("Invalid event signature format: could not split into name and parameters")
		return "", fmt.Errorf("invalid event signature format")
	}

	eventName := parts[0]
	params := strings.TrimSuffix(parts[1], ")")
	log.Printf("Event name: %s, Parameters: %s", eventName, params)

	// Split parameters and process each one
	paramList := strings.Split(params, ",")
	for i, param := range paramList {
		// Check if the parameter has the indexed keyword
		if strings.Contains(param, "indexed") {
			// Extract the type part (everything before "indexed")
			typePart := strings.Split(param, "indexed")[0]
			// If there's a parameter name after "indexed", remove it
			if strings.Contains(typePart, " ") {
				typePart = strings.Split(typePart, " ")[0]
			}
			paramList[i] = typePart
			log.Printf("Parameter %d: %s -> %s", i, param, paramList[i])
		} else if strings.Contains(param, " ") {
			// If parameter has a name but no indexed keyword, take only the type part
			paramList[i] = strings.Split(param, " ")[0]
			log.Printf("Parameter %d: %s -> %s", i, param, paramList[i])
		}
	}

	// Create the canonical signature without parameter names and with proper indexed handling
	canonicalSig := fmt.Sprintf("%s(%s)", eventName, strings.Join(paramList, ","))
	log.Printf("Canonical signature: %s", canonicalSig)

	// Calculate the keccak256 hash of the canonical signature
	hash := crypto.Keccak256Hash([]byte(canonicalSig))
	log.Printf("Event signature hash: %s", hash.Hex())

	// Return the hash as a hex string with 0x prefix
	return hash.Hex(), nil
}
