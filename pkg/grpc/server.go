package grpc

import (
	"fmt"
	"net"
	"sync"

	pb "event-pool/internal/proto"

	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedEventServiceServer
	mu          sync.RWMutex
	subscribers map[string][]chan *pb.Event
	grpcServer  *grpc.Server
}

func NewServer() *Server {
	return &Server{
		subscribers: make(map[string][]chan *pb.Event),
		grpcServer:  grpc.NewServer(),
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
