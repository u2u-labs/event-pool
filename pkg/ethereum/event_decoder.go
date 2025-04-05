package ethereum

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// EventDecoder decodes Ethereum event logs into human-readable JSON
type EventDecoder struct {
	// Map of event signatures to their ABI definitions
	eventABIs map[string]abi.Event
}

// NewEventDecoder creates a new EventDecoder
func NewEventDecoder() *EventDecoder {
	return &EventDecoder{
		eventABIs: make(map[string]abi.Event),
	}
}

// RegisterEvent registers an event signature with its ABI definition
func (d *EventDecoder) RegisterEvent(eventSignature string, eventABI string) error {
	// Parse the event ABI
	parsedABI, err := abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		return fmt.Errorf("failed to parse event ABI: %w", err)
	}

	// Get the first event (we assume there's only one in the ABI)
	if len(parsedABI.Events) == 0 {
		return fmt.Errorf("no events found in ABI")
	}

	// Get the first event from the map
	var event abi.Event
	for _, e := range parsedABI.Events {
		event = e
		break
	}

	// Store the event ABI
	d.eventABIs[eventSignature] = event

	log.Printf("Registered event %s with signature %s", event.Name, eventSignature)
	return nil
}

// DecodeEvent decodes an event log into a human-readable JSON string
func (d *EventDecoder) DecodeEvent(eventSignature string, data []byte, topics []common.Hash) (string, error) {
	// Get the event ABI
	event, ok := d.eventABIs[eventSignature]
	if !ok {
		// If we don't have the ABI, return the hex data
		return fmt.Sprintf("{\"raw\": \"%s\"}", common.Bytes2Hex(data)), nil
	}

	// Create a map to store the decoded data
	decodedData := make(map[string]interface{})

	// Decode non-indexed parameters from data
	if len(data) > 0 {
		// Create a map to store the decoded data
		unpacked := make(map[string]interface{})

		// Unpack the data
		err := event.Inputs.UnpackIntoMap(unpacked, data)
		if err != nil {
			log.Printf("Error unpacking event data: %v", err)
			return fmt.Sprintf("{\"raw\": \"%s\", \"error\": \"%v\"}", common.Bytes2Hex(data), err), nil
		}

		// Add non-indexed parameters to the decoded data
		for name, value := range unpacked {
			// Convert big.Int to string for better readability
			if bigInt, ok := value.(*big.Int); ok {
				decodedData[name] = bigInt.String()
			} else if addr, ok := value.(common.Address); ok {
				decodedData[name] = addr.Hex()
			} else {
				decodedData[name] = value
			}
		}
	}

	// Decode indexed parameters from topics
	// The first topic is always the event signature
	if len(topics) > 1 {
		// Get indexed parameters
		indexedParams := make([]abi.Argument, 0)
		for _, input := range event.Inputs {
			if input.Indexed {
				indexedParams = append(indexedParams, input)
			}
		}

		// Decode indexed parameters
		for i, param := range indexedParams {
			if i+1 >= len(topics) {
				break
			}

			// Get the topic
			topic := topics[i+1]

			// Decode the topic based on the parameter type
			var value interface{}
			var err error

			switch param.Type.T {
			case abi.AddressTy:
				value = common.BytesToAddress(topic.Bytes())
			case abi.BoolTy:
				value = topic.Big().Cmp(big.NewInt(0)) != 0
			case abi.IntTy, abi.UintTy:
				value = topic.Big().String()
			default:
				// For other types, just use the hex string
				value = topic.Hex()
			}

			if err != nil {
				log.Printf("Error decoding topic %d: %v", i, err)
				continue
			}

			// Add the decoded value to the map
			decodedData[param.Name] = value
		}
	}

	// Convert the decoded data to JSON
	jsonData, err := json.MarshalIndent(decodedData, "", "\t")
	if err != nil {
		log.Printf("Error marshaling decoded data to JSON: %v", err)
		return fmt.Sprintf("{\"raw\": \"%s\", \"error\": \"%v\"}", common.Bytes2Hex(data), err), nil
	}

	return string(jsonData), nil
}
