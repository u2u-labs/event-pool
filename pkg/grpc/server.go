package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"event-pool/contracts/sessionreceipt"
	crypto2 "event-pool/crypto"
	ws2 "event-pool/helper/ws"
	pb "event-pool/internal/proto"
	"event-pool/network/common"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"google.golang.org/grpc"
)

const LOGIN_MESSAGE = "logmein"

type Server struct {
	pb.UnimplementedEventServiceServer
	mu              sync.RWMutex
	subscribers     map[string][]chan *pb.Event
	activeConns     map[string]*activeConnection // Added to track active connections
	grpcServer      *grpc.Server
	db              *db.PrismaClient
	secretKey       []byte
	sessionContract string
	client          *ethereum.Client
	logger          *zap.SugaredLogger
}

// activeConnection tracks both the channel and the websocket connection
type activeConnection struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	logger *zap.SugaredLogger
}

func NewServer(db *db.PrismaClient, secretKey string, sessionContract string, client *ethereum.Client, logger *zap.SugaredLogger) *Server {
	return &Server{
		subscribers:     make(map[string][]chan *pb.Event),
		grpcServer:      grpc.NewServer(),
		activeConns:     make(map[string]*activeConnection),
		db:              db,
		secretKey:       []byte(secretKey),
		sessionContract: sessionContract,
		client:          client,
		logger:          logger,
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	secret := r.Header.Get("X-Secret")
	if s.secretKey != nil && secret != string(s.secretKey) {
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
	token := req.URL.Query().Get("token")
	claims, err := s.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	key := fmt.Sprintf("%s/%s/%s", chainId, contractAddress, eventName)

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
			s.logger.Infoln("Connection expired")
			go s.submitSessionReceipt(claims)
			return
		case <-ctx.Done():
			s.logger.Infoln("Connection terminated by server")
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

			if sendErr := wrapConn.WriteMessage(msgType, Data(data)); sendErr != nil {
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
		s.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
	}

	// Close the connection
	if err := activeConn.conn.Close(); err != nil {
		s.logger.Infoln(fmt.Sprintf("Error closing connection: %s", err.Error()))
	}
	delete(s.activeConns, token)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"disconnected"}`))

	s.logger.Infof("Disconnected client with address %s\n", claims.Address)

	go s.submitSessionReceipt(claims)
}

// submitSessionReceipt handles submitting the session receipt data to the blockchain
func (s *Server) submitSessionReceipt(claims *JwtClaims) {
	// submit receipt data
	now := time.Now().Unix()
	iss, err := claims.GetIssuedAt()
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error getting issuer: %s", err.Error()))
		return
	}

	// Load private key for signing
	secretBytes, err := os.ReadFile(filepath.Join(viper.GetString("data_dir"), "consensus/validator.key"))
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error reading validator key: %s", err.Error()))
		return
	}
	privateKey, err := crypto2.BytesToECDSAPrivateKey(secretBytes)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error converting validator key to ECDSA: %s", err.Error()))
		return
	}

	// Create a new transactor with the private key
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(s.client.GetChainId())))
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error creating transactor: %s", err.Error()))
		return
	}

	// Get current gas price
	gasPrice, err := s.client.GetClient().SuggestGasPrice(context.Background())
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error getting gas price: %s", err.Error()))
		return
	}

	// Set transaction parameters
	auth.GasPrice = gasPrice
	auth.GasLimit = uint64(3000000) // Set appropriate gas limit

	sessionReceipt, err := sessionreceipt.NewSessionReceipt(common2.HexToAddress(s.sessionContract), s.client.GetClient())
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error creating session receipt: %s", err.Error()))
		return
	}

	nonce, err := sessionReceipt.GetNonce(nil, common2.HexToAddress(claims.Address))
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error getting nonce: %s", err.Error()))
		return
	}

	// Call CreateSessionReceipt with the auth object to sign and send the transaction
	tx, err := sessionReceipt.CreateSessionReceipt(
		auth,
		common2.HexToAddress(claims.Address),
		big.NewInt(now-iss.Unix()),
		common2.HexToAddress("0x0000000000000000000000000000000000000000"),
		0,
		nonce,
	)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error creating session receipt: %s", err.Error()))
		return
	}

	s.logger.Infof("Session receipt transaction sent: %s\n", tx.Hash().Hex())

	// Wait for the transaction to be mined
	receipt, err := bind.WaitMined(context.Background(), s.client.GetClient(), tx)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error waiting for transaction to be mined: %s", err.Error()))
		return
	}

	if receipt.Status == types.ReceiptStatusSuccessful {
		s.logger.Infof("Session receipt transaction successful, block: %d\n", receipt.BlockNumber)
	} else {
		s.logger.Infoln("Session receipt transaction failed")
	}
}

type JwtClaims struct {
	Address string `json:"address"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed JWT token
func (s *Server) GenerateJWT(address string, expiry int64) (string, error) {
	claims := JwtClaims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiry) * time.Second)),
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
		return false
	}

	signature[crypto.RecoveryIDOffset] -= 27 // Transform yellow paper V from 27/28 to 0/1

	messageHash := accounts.TextHash([]byte(message))

	pubKey, err := crypto.SigToPub(messageHash, signature)
	if err != nil {
		return false
	}

	return common2.HexToAddress(fromAddress) == crypto.PubkeyToAddress(*pubKey)
}

func (a *activeConnection) Close() error {
	// Trigger graceful disconnection
	a.cancel()

	// Send close frame to client
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by server")
	if err := a.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		a.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
	}

	// Close the connection
	if err := a.conn.Close(); err != nil {
		a.logger.Infoln(fmt.Sprintf("Error closing connection: %s", err.Error()))
	}

	return nil
}
