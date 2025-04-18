package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"event-pool/internal/config"
	"event-pool/internal/worker"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"
)

type ContractHandler struct {
	db         *db.PrismaClient
	worker     *worker.Worker
	config     *config.Config
	ethClients map[int]*ethereum.Client
}

func NewContractHandler(db *db.PrismaClient, worker *worker.Worker, config *config.Config, ethClients map[int]*ethereum.Client) *ContractHandler {
	return &ContractHandler{
		db:         db,
		worker:     worker,
		config:     config,
		ethClients: ethClients,
	}
}

type RegisterContractRequest struct {
	ChainID        int    `json:"chainId"`
	ContractAddr   string `json:"contractAddress"`
	EventSignature string `json:"eventSignature"`
	EventABI       string `json:"eventAbi,omitempty"`
	StartBlock     int    `json:"startBlock"`
}

type RegisterContractResponse struct {
	ID string `json:"id"`
}

func (h *ContractHandler) RegisterContract(w http.ResponseWriter, r *http.Request) {
	var req RegisterContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received contract registration request: ChainID=%d, ContractAddr=%s, EventSig=%s, StartBlock=%d",
		req.ChainID, req.ContractAddr, req.EventSignature, req.StartBlock)

	// Validate chain ID is supported
	if _, ok := h.config.Ethereum.Chains[req.ChainID]; !ok {
		log.Printf("Unsupported chain ID: %d", req.ChainID)
		http.Error(w, fmt.Sprintf("Unsupported chain ID: %d", req.ChainID), http.StatusBadRequest)
		return
	}

	// Validate contract address format
	if !ethereum.IsValidAddress(req.ContractAddr) {
		log.Printf("Invalid contract address format: %s", req.ContractAddr)
		http.Error(w, "Invalid contract address format", http.StatusBadRequest)
		return
	}

	// Convert readable event signature to proper signature format
	eventSig, err := ethereum.ParseEventSignature(req.EventSignature)
	if err != nil {
		log.Printf("Error parsing event signature: %v", err)
		http.Error(w, fmt.Sprintf("Invalid event signature format: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Parsed event signature: %s -> %s", req.EventSignature, eventSig)

	eventName, err := ethereum.ExtractEventName(req.EventSignature)
	if err != nil {
		log.Printf("Error extracting event name: %v", err)
		http.Error(w, fmt.Sprintf("Invalid event signature format: %v", err), http.StatusBadRequest)
		return
	}

	contract, err := h.db.Contract.CreateOne(
		db.Contract.ChainID.Set(req.ChainID),
		db.Contract.EventName.Set(eventName),
		db.Contract.Address.Set(strings.ToLower(req.ContractAddr)),
		db.Contract.EventSignature.Set(eventSig),
		db.Contract.StartBlock.Set(req.StartBlock),
		db.Contract.EventABI.Set(req.EventABI),
	).Exec(r.Context())

	if err != nil {
		log.Printf("Error creating contract in database: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create contract: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Created contract in database with ID: %s", contract.ID)

	// Register the event ABI with the decoder if provided
	if req.EventABI != "" {
		client, ok := h.ethClients[req.ChainID]
		if !ok {
			log.Printf("No Ethereum client found for chain ID %d", req.ChainID)
			http.Error(w, fmt.Sprintf("No Ethereum client found for chain ID %d", req.ChainID), http.StatusInternalServerError)
			return
		}

		// Register the event ABI with the decoder
		err = client.RegisterEventABI(eventSig, req.EventABI)
		if err != nil {
			log.Printf("Error registering event ABI: %v", err)
			// Continue anyway, as this is not critical
		} else {
			log.Printf("Registered event ABI for signature %s", eventSig)
		}
	}

	// Create backfill task
	payload := worker.BackfillPayload{
		ChainID:      req.ChainID,
		ContractAddr: req.ContractAddr,
		EventSig:     eventSig,
		StartBlock:   int64(req.StartBlock),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling backfill payload: %v", err)
		http.Error(w, fmt.Sprintf("Failed to marshal payload: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Enqueueing backfill task with payload: %+v", payload)

	if err := h.worker.EnqueueTask(worker.TypeBackfill, payloadBytes); err != nil {
		log.Printf("Error enqueueing backfill task: %v", err)
		http.Error(w, fmt.Sprintf("Failed to enqueue backfill task: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully enqueued backfill task for contract %s", req.ContractAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RegisterContractResponse{ID: contract.ID})
}

func (h *ContractHandler) GetContracts(w http.ResponseWriter, r *http.Request) {
	chainIDStr := r.URL.Query().Get("chainId")
	address := r.URL.Query().Get("address")
	eventSig := r.URL.Query().Get("eventSignature")

	var filters []db.ContractWhereParam

	if chainIDStr != "" {
		chainID, err := strconv.Atoi(chainIDStr)
		if err != nil {
			http.Error(w, "Invalid chain ID", http.StatusBadRequest)
			return
		}
		filters = append(filters, db.Contract.ChainID.Equals(chainID))
	}

	if address != "" {
		filters = append(filters, db.Contract.Address.Equals(address))
	}

	if eventSig != "" {
		filters = append(filters, db.Contract.EventSignature.Equals(eventSig))
	}

	contracts, err := h.db.Contract.FindMany(
		filters...,
	).Exec(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch contracts: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contracts)
}

func (h *ContractHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	contractAddress := r.URL.Query().Get("contractAddress")
	chainIdStr := r.URL.Query().Get("chainId")
	txHash := r.URL.Query().Get("txHash")
	pageStr := r.URL.Query().Get("take")
	limitStr := r.URL.Query().Get("skip")

	if contractAddress == "" {
		http.Error(w, "contractAddress is required", http.StatusBadRequest)
		return
	}

	take := 10
	skip := 0
	chainId := 39

	if chainIdStr != "" {
		if c, err := strconv.Atoi(chainIdStr); err == nil {
			chainId = c
		} else {
			http.Error(w, "invalid chainId parameter", http.StatusBadRequest)
			return
		}
	}
	if pageStr != "" {
		if t, err := strconv.Atoi(pageStr); err == nil && t > 0 {
			take = t
		} else {
			http.Error(w, "invalid page parameter", http.StatusBadRequest)
			return
		}
	}

	if limitStr != "" {
		if s, err := strconv.Atoi(limitStr); err == nil && s > 0 {
			skip = s
		} else {
			http.Error(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
	}
	fmt.Println("chainId", chainId)
	contract, err := h.db.Contract.FindFirst(
		db.Contract.ChainID.Equals(chainId),
		db.Contract.Address.Equals(strings.ToLower(contractAddress)),
	).Exec(r.Context())

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find contract: %v", err), http.StatusInternalServerError)
		return
	}

	if contract == nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}

	var filters []db.EventLogWhereParam

	filters = append(filters, db.EventLog.ContractID.Equals(contract.ID))

	if txHash != "" {
		filters = append(filters, db.EventLog.TxHash.Equals(strings.ToLower(txHash)))
	}

	events, err := h.db.EventLog.FindMany(
		filters...,
	).With(db.EventLog.Contract.Fetch()).OrderBy(db.EventLog.BlockNumber.Order(db.SortOrderDesc)).Skip(skip).Take(take).Exec(r.Context())

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query events: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data": events,
		"pagination": map[string]interface{}{
			"take": take,
			"skip": skip,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
