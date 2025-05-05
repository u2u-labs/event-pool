package grpc

import (
	"context"
	"encoding/json"
	ws2 "event-pool/helper/ws"
	"event-pool/network/common"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	pb "event-pool/internal/proto"
	"event-pool/prisma/db"

	"google.golang.org/grpc"
)

const LOGIN_MESSAGE = "logmein"

type Server struct {
	pb.UnimplementedEventServiceServer
	mu          sync.RWMutex
	subscribers map[string][]chan *pb.Event
	activeConns map[string]*activeConnection // Added to track active connections
	grpcServer  *grpc.Server
	db          *db.PrismaClient
	secretKey   []byte
}

// activeConnection tracks both the channel and the websocket connection
type activeConnection struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
}

func NewServer(db *db.PrismaClient, secretKey string) *Server {
	return &Server{
		subscribers: make(map[string][]chan *pb.Event),
		grpcServer:  grpc.NewServer(),
		activeConns: make(map[string]*activeConnection),
		db:          db,
		secretKey:   []byte(secretKey),
	}
}

func (s *Server) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	pb.RegisterEventServiceServer(s.grpcServer, s)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.grpcServer.Stop()
}

func (s *Server) StreamEvents(req *pb.StreamEventsRequest, stream pb.EventService_StreamEventsServer) error {
	key := fmt.Sprintf("%d/%s/%s", req.ChainId, req.ContractAddress, req.EventSignature)

	// Create a channel for this subscriber
	eventChan := make(chan *pb.Event, 100)

	// Register the subscriber
	s.mu.Lock()
	s.subscribers[key] = append(s.subscribers[key], eventChan)
	s.mu.Unlock()

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
		s.mu.Unlock()
		close(eventChan)
	}()

	// Stream events to the client
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event := <-eventChan:
			if err := stream.Send(event); err != nil {
				return fmt.Errorf("failed to send event: %v", err)
			}
		}
	}
}

func (s *Server) BroadcastEvent(chainID int32, contractAddr string, eventSignature string, event *pb.Event) error {
	key := fmt.Sprintf("%d/%s/%s", chainID, contractAddr, eventSignature)

	s.mu.RLock()
	subscribers := s.subscribers[key]
	s.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// Channel is full, skip this event
		}
	}

	return nil
}

func (s *Server) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	if req.ContractAddress == "" {
		return nil, fmt.Errorf("contractAddress is required")
	}

	take := 10
	skip := 0
	chainId := 39 // default chain ID

	if req.ChainId != 0 {
		chainId = int(req.ChainId)
	}

	if req.Take > 0 {
		take = int(req.Take)
	}

	if req.Skip > 0 {
		skip = int(req.Skip)
	}

	contract, err := s.db.Contract.FindFirst(
		db.Contract.ChainID.Equals(chainId),
		db.Contract.Address.Equals(strings.ToLower(req.ContractAddress)),
	).Exec(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find contract: %v", err)
	}

	if contract == nil {
		return nil, fmt.Errorf("contract not found")
	}

	var filters []db.EventLogWhereParam
	filters = append(filters, db.EventLog.ContractID.Equals(contract.ID))

	if req.TxHash != "" {
		filters = append(filters, db.EventLog.TxHash.Equals(strings.ToLower(req.TxHash)))
	}

	events, err := s.db.EventLog.FindMany(
		filters...,
	).With(
		db.EventLog.Contract.Fetch(),
	).OrderBy(
		db.EventLog.BlockNumber.Order(db.SortOrderDesc),
	).Skip(skip).Take(take).Exec(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query events: %v", err)
	}

	response := &pb.GetEventsResponse{
		Data: make([]*pb.EventData, 0, len(events)),
		Pagination: &pb.Pagination{
			Skip: int32(skip),
			Take: int32(take),
		},
	}

	for _, event := range events {
		contract := event.Contract()
		eventData := &pb.EventData{
			Id:              event.ID,
			ContractAddress: contract.Address,
			BlockNumber:     int64(event.BlockNumber),
			TxHash:          event.TxHash,
			LogIndex:        int32(event.LogIndex),
			Data:            event.Data,
			CreatedAt:       event.CreatedAt.Format(time.RFC3339),
		}
		response.Data = append(response.Data, eventData)
	}

	return response, nil
}

func (s *Server) RequestToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address         string `json:"address"`
		Signature       string `json:"signature"`
		ChainId         string `json:"chain_id"`
		Timestamp       int64  `json:"timestamp"`
		ContractAddress string `json:"contract_address"`
		EventSignature  string `json:"event_signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	const allowedSkew = 5 * time.Minute

	timestampTime := time.Unix(body.Timestamp, 0)
	now := time.Now()

	if now.Sub(timestampTime) > allowedSkew || timestampTime.After(now.Add(allowedSkew)) {
		http.Error(w, "Invalid or expired timestamp", http.StatusUnauthorized)
		return
	}
	if !VerifySignature(body.Address, fmt.Sprintf("%s_%d", LOGIN_MESSAGE, body.Timestamp), body.Signature) {
		fmt.Println(fmt.Sprintf("Invalid signature"))
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	key := fmt.Sprintf("%s/%s/%s", body.ChainId, body.ContractAddress, body.EventSignature)
	token, err := s.GenerateJWT(body.Address, body.ChainId, body.ContractAddress, body.EventSignature, key, 30*24*60) // 30 days
	if err != nil {
		fmt.Println(fmt.Sprintf("Unable to generate JWT, %s", err.Error()))
		http.Error(w, "Unable to generate JWT", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
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
	// CORS rule - Allow requests from anywhere
	wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// Upgrade the connection to a WS one
	ws, err := wsUpgrader.Upgrade(w, req, nil)
	if err != nil {
		fmt.Println(fmt.Sprintf("Unable to upgrade to a WS connection, %s", err.Error()))

		return
	}

	token := req.URL.Query().Get("token")
	claims, err := s.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	key := claims.Key

	// Create a context with cancel for this connection
	ctx, cancel := context.WithCancel(context.Background())

	// Create a channel for this subscriber
	eventChan := make(chan *pb.Event, 100)

	// Register the subscriber
	s.mu.Lock()
	s.subscribers[key] = append(s.subscribers[key], eventChan)
	s.activeConns[token] = &activeConnection{
		conn:   ws,
		cancel: cancel,
	}
	s.mu.Unlock()

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
		delete(s.activeConns, token)
		s.mu.Unlock()
		close(eventChan)
		cancel()
	}()

	// Defer WS closure
	defer func(ws *websocket.Conn) {
		err = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			fmt.Println(fmt.Sprintf("Error sending close message: %s", err.Error()))
		}

		err = ws.Close()
		if err != nil {
			fmt.Println(
				fmt.Sprintf("Unable to gracefully close WS connection, %s", err.Error()),
			)
		}
	}(ws)

	wrapConn := &ws2.WsWrapper{Ws: ws, Logger: common.NewNullSugaredLogger()}

	fmt.Println("Websocket connection established")
	// Run the listen loop

	// Send the token to the client
	wrapConn.WriteMessage(websocket.TextMessage, []byte("Returning streaming token:"))
	wrapConn.WriteMessage(websocket.TextMessage, Data(token))

	// Run a separate goroutine to listen for context cancellation
	go func() {
		<-ctx.Done()
		// This will be triggered when cancel() is called
		return
	}()

	for {
		// Check if context is done (disconnection requested)
		select {
		case <-ctx.Done():
			fmt.Println("Connection terminated by server")
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
				fmt.Println("Closing WS connection gracefully")
			} else {
				fmt.Println(fmt.Sprintf("Unable to read WS message, %s", err.Error()))
				fmt.Println("Closing WS connection with error")
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

			if sendErr := wrapConn.WriteMessage(msgType, Data(data)); sendErr != nil {
				fmt.Println(fmt.Sprintf("Unable to write WS message, %s", err.Error()))
				return
			}
		case <-time.After(100 * time.Millisecond):
			// This prevents the select from blocking
			continue
		}
	}
}

func (s *Server) DisconnectWs(w http.ResponseWriter, req *http.Request) {
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
	// Trigger graceful disconnection
	activeConn.cancel()

	// Send close frame to client
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by server")
	if err := activeConn.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		fmt.Println(fmt.Sprintf("Error sending close message: %s", err.Error()))
	}

	// Close the connection
	if err := activeConn.conn.Close(); err != nil {
		fmt.Println(fmt.Sprintf("Error closing connection: %s", err.Error()))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"disconnected"}`))

	fmt.Printf("Disconnected client with address %s for key %s\n", claims.Address, claims.Key)

}

type JwtClaims struct {
	Address         string `json:"address"`
	ChainId         string `json:"chain_id"`
	ContractAddress string `json:"contract_address"`
	EventSignature  string `json:"event_signature"`
	Key             string `json:"key"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed JWT token
func (s *Server) GenerateJWT(address, chainId, contractAddress, eventSignature, key string, expirationMinutes int) (string, error) {
	claims := JwtClaims{
		Address:         address,
		Key:             key,
		ChainId:         chainId,
		ContractAddress: contractAddress,
		EventSignature:  eventSignature,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expirationMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// ValidateJWT parses and verifies a JWT token
func (s *Server) ValidateJWT(tokenStr string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*JwtClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func VerifySignature(fromAddress, message, signatureHex string) bool {
	signature, err := hexutil.Decode(signatureHex)
	if err != nil {
		fmt.Println("failed to decode", "err", err)
		return false
	}

	signature[crypto.RecoveryIDOffset] -= 27 // Transform yellow paper V from 27/28 to 0/1

	messageHash := accounts.TextHash([]byte(message))

	pubKey, err := crypto.SigToPub(messageHash, signature)
	if err != nil {
		fmt.Println("failed to parse", "err", err)
		return false
	}

	return common2.HexToAddress(fromAddress) == crypto.PubkeyToAddress(*pubKey)
}
