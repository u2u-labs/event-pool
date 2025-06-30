package grpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"event-pool/internal/jwt"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

func (s *Server) RequestTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	secret := r.Header.Get("X-Secret")
	if s.gatewaySecret != nil && secret != string(s.gatewaySecret) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Address  string `json:"address"`
		Duration int64  `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := jwt.GenerateJWT(body.Address, body.Duration, s.jwtSecret)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Unable to generate JWT, %s", err.Error()))
		http.Error(w, "Unable to generate JWT", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_at": time.Now().Add(time.Duration(body.Duration) * time.Second).Unix(),
	})
	return
}

// wsUpgrader defines upgrade parameters for the WS connection
var wsUpgrader = websocket.Upgrader{
	// Uses the default HTTP buffer sizes for Read / Write buffers.
	// Documentation specifies that they are 4096B in size.
	// There is no need to have them be 4x in size when requests / responses
	// shouldn't exceed 1024B
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) GetContractStatus(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	chainId := vars["chainId"]
	address := vars["address"]
	eventNameString := vars["eventName"]
	if chainId == "" || address == "" || eventNameString == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	chainIdNumber, err := strconv.ParseInt(chainId, 10, 64)
	if err != nil {
		http.Error(w, "Invalid chainId", http.StatusBadRequest)
		return
	}

	// extract event name from event signature
	eventName := eventNameString
	if strings.Contains(eventNameString, "(") {
		eventName, err = ethereum.ExtractEventName(eventNameString)
		if err != nil {
			s.logger.Infof("Error extracting event name: %v", err)
			http.Error(w, fmt.Sprintf("Invalid event signature format: %v", err), http.StatusBadRequest)
			return
		}
	}

	contractInfo, err := s.db.Contract.FindFirst(
		db.Contract.ChainID.Equals(int(chainIdNumber)),
		db.Contract.Address.Equals(strings.ToLower(address)),
		db.Contract.EventName.Equals(eventName)).Exec(req.Context())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "Contract not found", http.StatusNotFound)
			return
		}
		s.logger.Errorw("Error querying contract status", "err", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	lastBlock, err := s.GetMonitorLastBlock(int(chainIdNumber))
	if err != nil {
		http.Error(w, "Chain not supported", http.StatusBadRequest)
		return
	}
	currentBlockNumber := lastBlock

	currentIndexedBackfillNumberKey := strings.ToLower(fmt.Sprintf("backfill_status:%s/%s/%s", chainId, address, contractInfo.EventSignature))
	data, ok := s.metrics.Load(currentIndexedBackfillNumberKey)
	if ok {
		currentBlock, ok := data.(*big.Int)
		if !ok {
			s.logger.Errorw("Error getting current block number", "err", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		currentBlockNumber = currentBlock.Uint64()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"chainId":            chainId,
		"contractAddress":    address,
		"eventName":          eventName,
		"currentBlockNumber": currentBlockNumber,
		"lastBlockNumber":    lastBlock,
	})
}
