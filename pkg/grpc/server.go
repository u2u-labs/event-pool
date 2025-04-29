package grpc

import (
	"context"
	"encoding/json"
	ws2 "event-pool/helper/ws"
	"event-pool/network/common"
	"fmt"
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

type Server struct {
	pb.UnimplementedEventServiceServer
	mu          sync.RWMutex
	subscribers map[string][]chan *pb.Event
	grpcServer  *grpc.Server
	db          *db.PrismaClient
}

func NewServer(db *db.PrismaClient) *Server {
	return &Server{
		subscribers: make(map[string][]chan *pb.Event),
		grpcServer:  grpc.NewServer(),
		db:          db,
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

	chainId := req.URL.Query().Get("chain_id")
	contractAddr := req.URL.Query().Get("contract_address")
	eventSignature := req.URL.Query().Get("event_signature")
	token := req.URL.Query().Get("token")
	key := fmt.Sprintf("%s/%s/%s", chainId, contractAddr, eventSignature)

	// check if the token is valid
	if err = s.validateToken(token); err != nil {
		fmt.Println(fmt.Sprintf("Invalid token, %s", err.Error()))
		return
	}

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

	cancel := func() {
	}

	// Defer WS closure
	defer func(ws *websocket.Conn) {
		cancel()
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

	for {
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

			resp, err := json.Marshal(data)
			if err != nil {
				fmt.Println(fmt.Sprintf("Unable to marshal WS message, %s", err.Error()))
				return
			}
			if sendErr := wrapConn.WriteMessage(msgType, resp); sendErr != nil {
				return
			}
		}
	}
}

// TODO: define the token validation logic
func (s *Server) validateToken(token string) error {
	if token == "" {
		return fmt.Errorf("token is required")
	}

	return nil
}
