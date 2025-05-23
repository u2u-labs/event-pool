package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	ws2 "event-pool/helper/ws"
	pb "event-pool/internal/proto"
	"event-pool/network/common"
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

	token, err := s.GenerateJWT(body.Address, body.Duration)
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

func (s *Server) HandleWs(w http.ResponseWriter, req *http.Request) {
	secret := req.Header.Get("X-Secret")
	if s.gatewaySecret != nil && secret != string(s.gatewaySecret) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := req.URL.Query().Get("token")
	claims, err := s.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// CORS rule - Allow requests from anywhere
	wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// Upgrade the connection to a WS one
	ws, err := wsUpgrader.Upgrade(w, req, nil)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Unable to upgrade to a WS connection, %s", err.Error()))

		return
	}

	chainId := req.URL.Query().Get("chain_id")
	contractAddress := req.URL.Query().Get("contract_address")
	eventName := req.URL.Query().Get("event_name")
	key := strings.ToLower(fmt.Sprintf("%s/%s/%s", chainId, contractAddress, eventName))

	// Create a context with cancel for this connection
	ctx, cancel := context.WithCancel(context.Background())

	// Create a channel for this subscriber
	eventChan := make(chan *pb.Event, 100)

	// Register the subscriber
	s.mu.Lock()
	oldConn, exist := s.activeConns[token]
	if exist {
		oldConn.Close()
	}

	s.subscribers[key] = append(s.subscribers[key], eventChan)
	s.activeConns[token] = &activeConnection{
		conn:   ws,
		cancel: cancel,
		logger: s.logger.Named("ws"),
	}
	s.mu.Unlock()

	// Track if this was a clean disconnect
	cleanDisconnect := false

	// Cleanup when the stream ends
	defer func() {
		s.mu.Lock()
		subs := s.subscribers[key]
		for i, ch := range subs {
			if ch == eventChan {
				subs = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		s.subscribers[key] = subs
		if conn, exists := s.activeConns[token]; exists && conn.conn == ws {
			delete(s.activeConns, token)
		}
		s.mu.Unlock()
		close(eventChan)
		cancel()

		// If this wasn't a clean disconnect, schedule a session receipt
		if !cleanDisconnect {
			s.scheduler.ScheduleSessionReceipt(token, claims, ReceiptSubmissionDelay)
		}
	}()

	// Defer WS closure
	defer func(ws *websocket.Conn) {
		err = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			s.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
		}

		err = ws.Close()
		if err != nil {
			s.logger.Infoln(
				fmt.Sprintf("Unable to gracefully close WS connection, %s", err.Error()),
			)
		}
	}(ws)

	wrapConn := &ws2.WsWrapper{Ws: ws, Logger: common.NewNullSugaredLogger()}

	s.logger.Infoln("Websocket connection established")
	// Run the listen loop
	expiry, err := claims.GetExpirationTime()
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Unable to get expiration time, %s", err.Error()))
		return
	}
	timer := time.NewTimer(time.Until(expiry.Time))

	// Send the token to the client
	wrapConn.WriteMessage(websocket.TextMessage, []byte("Connected"))

	// Run a separate goroutine to listen for context cancellation
	go func() {
		<-ctx.Done()
		// This will be triggered when cancel() is called
		return
	}()

	for {
		// Check if context is done (disconnection requested)
		select {
		case <-timer.C:
			s.logger.Infow("Connection expired", "addr", claims.Address)
			cleanDisconnect = true // This is an expected disconnect
			go s.submitSessionReceipt(token, claims)
			return
		case <-ctx.Done():
			cleanDisconnect = true // This is an expected disconnect
			s.logger.Infow("Connection terminated by server", "addr", claims.Address)
			return
		default:
			// Continue normal operation
		}

		// Read the incoming message
		msgType, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseAbnormalClosure,
			) {
				// Accepted close codes
				s.logger.Infoln("Closing WS connection gracefully")
			} else {
				s.logger.Infoln(fmt.Sprintf("Unable to read WS message, %s", err.Error()))
				s.logger.Infoln("Closing WS connection with error")
			}

			break
		}

		select {
		case event := <-eventChan:
			data := map[string]any{
				"data":         event.Data,
				"tx_hash":      event.TxHash,
				"block_number": event.BlockNumber,
			}
			dataRaws := Data(data)

			// counting total bytes sent
			count := int64(len(event.Data))
			total, err := s.rdb.IncrBy(ctx, fmt.Sprintf("%s_%s", USAGE_BYTES_KEY, token), count).Result()
			if err != nil {
				s.logger.Warnw("Unable to increment user total bytes sent", "err", err.Error(), "addr", claims.Address)
			}

			// If total equals `count`, it means the key was just created
			if total == count {
				expireAt := time.Unix(claims.ExpiresAt.Unix(), 0).Add(10 * time.Minute)
				err = s.rdb.ExpireAt(ctx, fmt.Sprintf("%s_%s", USAGE_BYTES_KEY, token), expireAt).Err()
				if err != nil {
					s.logger.Warnw("Unable to set expiration on user byte count", "err", err.Error(), "addr", claims.Address)
				}
			}

			if sendErr := wrapConn.WriteMessage(msgType, dataRaws); sendErr != nil {
				s.logger.Infoln(fmt.Sprintf("Unable to write WS message, %s", err.Error()))
				return
			}
		case <-time.After(100 * time.Millisecond):
			// This prevents the select from blocking
			continue
		}
	}
}

func (s *Server) DisconnectWs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	secret := req.Header.Get("X-Secret")
	if s.gatewaySecret != nil && secret != string(s.gatewaySecret) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Token string `json:"token"`
	}

	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	token := body.Token
	claims, err := s.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	activeConn, exists := s.activeConns[token]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	// Cancel the scheduled receipt submission job for this token
	s.scheduler.CancelSessionReceipt(token)
	// Trigger graceful disconnection
	activeConn.cancel()

	// Send close frame to client
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by server")
	if err := activeConn.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		s.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
	}

	// Close the connection
	if err := activeConn.conn.Close(); err != nil {
		s.logger.Infoln(fmt.Sprintf("Error closing connection: %s", err.Error()))
	}
	delete(s.activeConns, token)

	// add token to blacklist
	_ = s.rdb.Set(context.Background(), fmt.Sprintf("%s_%s", BLACKLISTED_TOKEN_KEY, token), "true", claims.ExpiresAt.Sub(time.Now())).Err()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"disconnected"}`))

	s.logger.Infof("Disconnected client with address %s\n", claims.Address)

	go s.submitSessionReceipt(token, claims)
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

	key := strings.ToLower(fmt.Sprintf("backfill_status:%s/%s/%s", chainId, address, contractInfo.EventSignature))
	data, ok := s.metrics.Load(key)
	if ok {
		currentBlock, ok := data.(*big.Int)
		if !ok {
			s.logger.Errorw("Error getting current block number", "err", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"chainId":            chainId,
			"contractAddress":    address,
			"eventName":          eventName,
			"currentBlockNumber": currentBlock,
		})
		return
	}

	lastBlock, err := s.GetMonitorLastBlock(int(chainIdNumber))
	if err != nil {
		http.Error(w, "Chain not supported", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"chainId":            chainId,
		"contractAddress":    address,
		"eventName":          eventName,
		"currentBlockNumber": lastBlock,
	})
}
