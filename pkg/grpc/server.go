package grpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	nodestorage "event-pool/contracts/nodesstorage"
	"event-pool/contracts/sessionreceipt"
	crypto2 "event-pool/crypto"
	"event-pool/internal/jwt"
	pb "event-pool/internal/proto"
	"event-pool/pkg/ethereum"
	"event-pool/prisma/db"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"google.golang.org/grpc"
)

const (
	BLACKLISTED_TOKEN_KEY = "blacklist_token"
	USAGE_BYTES_KEY       = "usage_bytes"

	ReceiptSubmissionDelay = 5 * time.Minute
)

var (
	skippedMethods = map[string]bool{
		"/eventpool.EventService/RequestToken": true,
		// Add other RPC method names you want to skip JWT check
	}
)

type Server struct {
	pb.UnimplementedEventServiceServer
	mu            sync.RWMutex
	subscribers   map[string][]chan *pb.Event
	activeConns   map[string]*activeConnection // Added to track active connections
	grpcServer    *grpc.Server
	db            *db.PrismaClient
	gatewaySecret []byte
	jwtSecret     []byte
	client        *ethereum.Client
	rdb           *redis.Client
	metrics       *sync.Map
	scheduler     *SessionScheduler
	logger        *zap.SugaredLogger

	sessionContract string
	nodeContract    string

	GetMonitorLastBlock func(int) (uint64, error)
}

// activeConnection tracks both the channel and the websocket connection
type activeConnection struct {
	conn       *websocket.Conn
	grpcStream pb.EventService_StreamEventsServer
	cancel     context.CancelFunc
	logger     *zap.SugaredLogger
}

func NewServer(db *db.PrismaClient, gatewaySecretKey string, jwtSecret string, sessionContract string, nodeContract string, client *ethereum.Client, rdb *redis.Client, logger *zap.SugaredLogger) *Server {
	s := &Server{
		subscribers:     make(map[string][]chan *pb.Event),
		activeConns:     make(map[string]*activeConnection),
		db:              db,
		gatewaySecret:   []byte(gatewaySecretKey),
		jwtSecret:       []byte(jwtSecret),
		sessionContract: sessionContract,
		nodeContract:    nodeContract,
		client:          client,
		rdb:             rdb,
		metrics:         &sync.Map{},
		logger:          logger,
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(s.jwtUnaryInterceptor(skippedMethods)),
		grpc.StreamInterceptor(s.jwtStreamInterceptor(skippedMethods)),
	)
	s.grpcServer = grpcServer

	s.scheduler = NewSessionScheduler(func(token string, claims interface{}) {
		s.submitSessionReceipt(token, claims.(*jwt.JwtClaims))
	})

	return s
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
	s.scheduler.Stop()
	s.grpcServer.Stop()
	s.logger.Info("gRPC server stopped")
}

func (s *Server) GetMetrics() *sync.Map {
	return s.metrics
}

func (s *Server) RequestToken(ctx context.Context, req *pb.RequestTokenRequest) (*pb.RequestTokenResponse, error) {
	token, err := jwt.GenerateJWT(req.Address, req.Duration, s.jwtSecret)
	if err != nil {
		s.logger.Errorw(fmt.Sprintf("Unable to generate JWT, %s", err.Error()))
		return nil, status.Errorf(codes.Internal, "failed to generate JWT: %v", err)
	}

	return &pb.RequestTokenResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(req.Duration) * time.Second).Unix(),
	}, nil
}

func (s *Server) StreamEvents(req *pb.StreamEventsRequest, stream pb.EventService_StreamEventsServer) error {
	// Validate token from metadata
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return status.Error(codes.Unauthenticated, "missing token")
	}

	claims, err := s.ValidateJWT(tokens[0])
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	token := strings.TrimPrefix(tokens[0], "Bearer ")

	// Format the key the same way as in WebSocket handler
	key := strings.ToLower(fmt.Sprintf("%d/%s/%s", req.ChainId, req.ContractAddress, req.EventSignature))

	// Create a context with cancel for this connection
	ctx, cancel := context.WithCancel(stream.Context())

	// Create a channel for this subscriber
	eventChan := make(chan *pb.Event, 100)

	// Register the subscriber and track the connection
	s.mu.Lock()
	// Close any existing connection with the same token
	oldConn, exist := s.activeConns[token]
	if exist {
		oldConn.Close()
	}

	s.subscribers[key] = append(s.subscribers[key], eventChan)
	s.activeConns[token] = &activeConnection{
		grpcStream: stream,
		cancel:     cancel,
		logger:     s.logger.Named("grpc"),
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
		if conn, exists := s.activeConns[token]; exists && conn.grpcStream == stream {
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

	// Send initial connection confirmation
	initMetadata := metadata.New(map[string]string{"status": "Connected"})
	if err := stream.SendHeader(initMetadata); err != nil {
		return fmt.Errorf("failed to send header: %v", err)
	}

	// Check for token expiration
	expiry, err := claims.GetExpirationTime()
	if err != nil {
		return fmt.Errorf("unable to get expiration time: %v", err)
	}
	timer := time.NewTimer(time.Until(expiry.Time))

	// Stream events to the client
	for {
		select {
		case <-timer.C:
			s.logger.Infoln("Connection expired")
			cleanDisconnect = true // This is an expected disconnect
			go s.submitSessionReceipt(token, claims)
			return nil
		case <-ctx.Done():
			s.logger.Infoln("gRPC stream terminated by server")
			cleanDisconnect = true // This is an expected disconnect
			return nil
		case event := <-eventChan:
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

			if err := stream.Send(event); err != nil {
				s.logger.Infoln(fmt.Sprintf("Unable to send event: %s", err.Error()))
				// Error sending - likely client disconnected unexpectedly
				return err
			}
		}
	}
}

func (s *Server) BroadcastEvent(chainID int32, contractAddr string, eventSignature string, event *pb.Event) error {
	key := strings.ToLower(fmt.Sprintf("%d/%s/%s", chainID, contractAddr, eventSignature))

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

func (s *Server) DisconnectStream(ctx context.Context, req *pb.DisconnectStreamRequest) (*pb.DisconnectStreamResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	claims, err := s.ValidateJWT(tokens[0])
	if err != nil {
		return &pb.DisconnectStreamResponse{
			Success: false,
			Error:   "Invalid token",
		}, status.Error(codes.Unauthenticated, "Invalid token")
	}
	token := strings.TrimPrefix(tokens[0], "Bearer ")

	// Check if the connection exists
	s.mu.Lock()
	activeConn, exists := s.activeConns[token]
	s.mu.Unlock()

	if !exists {
		return &pb.DisconnectStreamResponse{
			Success: false,
			Error:   "Connection not found",
		}, status.Error(codes.NotFound, "Connection not found")
	}

	// Cancel the scheduled receipt submission job for this token
	s.scheduler.CancelSessionReceipt(token)

	// Trigger graceful disconnection
	activeConn.cancel()

	// Handle both gRPC and WS connection types
	if activeConn.conn != nil {
		// Send close frame to WebSocket client
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by server")
		if err := activeConn.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
			s.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
		}

		// Close the WebSocket connection
		if err := activeConn.conn.Close(); err != nil {
			s.logger.Infoln(fmt.Sprintf("Error closing connection: %s", err.Error()))
		}
	}

	// Remove the connection from active connections
	s.mu.Lock()
	delete(s.activeConns, token)
	s.mu.Unlock()

	// Add token to blacklist with expiration time based on the token's expiry
	_ = s.rdb.Set(ctx, fmt.Sprintf("%s_%s", BLACKLISTED_TOKEN_KEY, token), "true", claims.ExpiresAt.Sub(time.Now())).Err()

	s.logger.Infof("Disconnected client with address %s\n", claims.Address)

	// Submit session receipt asynchronously
	go s.submitSessionReceipt(token, claims)

	// Return success response
	return &pb.DisconnectStreamResponse{
		Success: true,
		Message: "Stream disconnected successfully",
	}, nil
}

func (s *Server) jwtUnaryInterceptor(skippedMethods map[string]bool) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract JWT token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "Missing metadata")
		}

		if skippedMethods[info.FullMethod] {
			secret := md["x-secret"]
			if len(secret) == 0 {
				return nil, status.Error(codes.Unauthenticated, "invalid x-secret")
			}
			if s.gatewaySecret != nil && secret[0] != string(s.gatewaySecret) {
				return nil, status.Error(codes.Unauthenticated, "invalid x-secret")
			}

			return handler(ctx, req)
		}

		tokens := md["authorization"]
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "Invalid or missing token")
		}
		claims, err := s.ValidateJWT(tokens[0])
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "Invalid token")
		}

		s.logger.Infow("Request received", "addr", claims.Address, "method", info.FullMethod)
		return handler(ctx, req)
	}
}

func (s *Server) jwtStreamInterceptor(skippedMethods map[string]bool) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if skippedMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "Missing metadata")
		}

		tokens := md["authorization"]
		if len(tokens) == 0 {
			return status.Error(codes.Unauthenticated, "Invalid or missing token")
		}
		// split the token into parts
		claims, err := s.ValidateJWT(tokens[0])
		if err != nil {
			return status.Error(codes.Unauthenticated, "Invalid token")
		}

		s.logger.Infow("Stream connected", "addr", claims.Address, "method", info.FullMethod)
		return handler(srv, ss)
	}
}

// submitSessionReceipt handles submitting the session receipt data to the blockchain
func (s *Server) submitSessionReceipt(token string, claims *jwt.JwtClaims) {
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

	addr := crypto.PubkeyToAddress(privateKey.PublicKey)
	nodeStorage, err := nodestorage.NewNodesStorage(common2.HexToAddress(s.nodeContract), s.client.GetClient())
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error connecting to node storage: %s", err.Error()))
		return
	}
	isValid, err := nodeStorage.IsValidNode(nil, addr)
	if err != nil {
		s.logger.Infoln(fmt.Sprintf("Error validating node address: %s", err.Error()))
		return
	}
	if !isValid {
		s.logger.Infoln(fmt.Sprintf("Invalid node address: %s", addr))
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

	totalBytesServed, err := s.rdb.Get(context.Background(), fmt.Sprintf("%s_%s", USAGE_BYTES_KEY, token)).Int64()
	if errors.Is(err, redis.Nil) {
		s.logger.Infow("User byte count key does not exist", "addr", claims.Address)
		totalBytesServed = 0
		return
	} else if err != nil {
		s.logger.Errorw("Error fetching total bytes served", "err", err.Error(), "addr", claims.Address)
		return
	}

	// Call CreateSessionReceipt with the auth object to sign and send the transaction
	tx, err := sessionReceipt.CreateSessionReceipt(
		auth,
		common2.HexToAddress(claims.Address),
		big.NewInt(totalBytesServed),
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
		_ = s.rdb.Del(context.Background(), fmt.Sprintf("%s_%s", USAGE_BYTES_KEY, token)).Err()
	} else {
		s.logger.Infoln("Session receipt transaction failed")
	}
}

// ValidateJWT parses and verifies a JWT token
func (s *Server) ValidateJWT(tokenStr string) (*jwt.JwtClaims, error) {
	claims, err := jwt.ValidateJWT(tokenStr, s.jwtSecret, func(token string) error {
		if err := s.rdb.Get(context.Background(), fmt.Sprintf("%s_%s", BLACKLISTED_TOKEN_KEY, tokenStr)).Err(); err == nil {
			return fmt.Errorf("token is invalid")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
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
	var wsErr, grpcErr error

	// Always trigger context cancellation
	if a.cancel != nil {
		a.cancel()
	}

	// Handle WebSocket connection if it exists
	if a.conn != nil {
		// Send close frame to WebSocket client
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Disconnected by server")
		if err := a.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
			a.logger.Infoln(fmt.Sprintf("Error sending close message: %s", err.Error()))
			wsErr = err
		}

		// Close the WebSocket connection
		if err := a.conn.Close(); err != nil {
			a.logger.Infoln(fmt.Sprintf("Error closing WebSocket connection: %s", err.Error()))
			if wsErr == nil {
				wsErr = err
			}
		}
	}

	// Handle gRPC stream if it exists
	if a.grpcStream != nil {
		// For gRPC, we don't need to explicitly close the stream as it will
		// be closed when the context is cancelled. However, we can try to send
		// a final metadata to indicate closure if needed.
		if stream, ok := a.grpcStream.(interface {
			SendHeader(metadata.MD) error
		}); ok {
			md := metadata.Pairs("status", "disconnected")
			if err := stream.SendHeader(md); err != nil {
				a.logger.Infoln(fmt.Sprintf("Error sending final metadata to gRPC stream: %s", err.Error()))
				grpcErr = err
			}
		}
		// The actual closing of the gRPC stream is handled by the context cancellation
	}

	// Return an error if either WebSocket or gRPC closure had an issue
	if wsErr != nil {
		return wsErr
	}
	if grpcErr != nil {
		return grpcErr
	}

	return nil
}
