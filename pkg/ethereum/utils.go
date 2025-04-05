package ethereum

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func HexToAddress(hex string) common.Address {
	// Ensure the hex string has the 0x prefix
	if !strings.HasPrefix(hex, "0x") {
		hex = "0x" + hex
	}
	return common.HexToAddress(hex)
}

func HexToHash(hex string) common.Hash {
	if !strings.HasPrefix(hex, "0x") {
		hex = "0x" + hex
	}
	return common.HexToHash(hex)
}

func IsValidAddress(address string) bool {
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	return common.IsHexAddress(address)
}

type Log = types.Log

func ParseEventSignature(readableSig string) (string, error) {
	log.Printf("Parsing event signature: %s", readableSig)

	readableSig = strings.ReplaceAll(readableSig, " ", "")
	log.Printf("After removing whitespace: %s", readableSig)

	if !strings.Contains(readableSig, "(") || !strings.HasSuffix(readableSig, ")") {
		log.Printf("Invalid event signature format: missing parentheses")
		return "", fmt.Errorf("invalid event signature format")
	}

	parts := strings.SplitN(readableSig, "(", 2)
	if len(parts) != 2 {
		log.Printf("Invalid event signature format: could not split into name and parameters")
		return "", fmt.Errorf("invalid event signature format")
	}

	eventName := parts[0]
	params := strings.TrimSuffix(parts[1], ")")
	log.Printf("Event name: %s, Parameters: %s", eventName, params)

	paramList := strings.Split(params, ",")
	for i, param := range paramList {

		if strings.Contains(param, "indexed") {

			typePart := strings.Split(param, "indexed")[0]

			if strings.Contains(typePart, " ") {
				typePart = strings.Split(typePart, " ")[0]
			}
			paramList[i] = typePart
			log.Printf("Parameter %d: %s -> %s", i, param, paramList[i])
		} else if strings.Contains(param, " ") {

			paramList[i] = strings.Split(param, " ")[0]
			log.Printf("Parameter %d: %s -> %s", i, param, paramList[i])
		}
	}

	canonicalSig := fmt.Sprintf("%s(%s)", eventName, strings.Join(paramList, ","))
	log.Printf("Canonical signature: %s", canonicalSig)

	hash := crypto.Keccak256Hash([]byte(canonicalSig))
	log.Printf("Event signature hash: %s", hash.Hex())

	return hash.Hex(), nil
}

func ExtractEventName(readableSig string) (string, error) {

	i := strings.Index(readableSig, "(")
	if i <= 0 || !strings.HasSuffix(readableSig, ")") {
		return "", errors.New("invalid function signature format")
	}
	return readableSig[:i], nil
}
