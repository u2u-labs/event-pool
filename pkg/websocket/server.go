package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	config   *Config
	upgrader websocket.Upgrader
	clients  map[string]*Client
	mu       sync.RWMutex
}

type Config struct {
	PingInterval   time.Duration
	PongWait       time.Duration
	WriteWait      time.Duration
	MaxMessageSize int64
}

type Client struct {
	server   *Server
	conn     *websocket.Conn
	send     chan []byte
	chainID  int
	contract string
	event    string
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: Implement proper origin checking
			},
		},
		clients: make(map[string]*Client),
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Parse URL path to get chainID, contract, and event
	// Format: /{chainId}/{contractAddress}/{event}
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 3 {
		log.Printf("Invalid WebSocket path format: %s", path)
		http.Error(w, "Invalid WebSocket path format. Expected: /{chainId}/{contractAddress}/{event}", http.StatusBadRequest)
		return
	}

	// Parse chain ID
	chainID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Printf("Invalid chain ID: %s", parts[0])
		http.Error(w, "Invalid chain ID", http.StatusBadRequest)
		return
	}

	contractAddr := parts[1]
	eventSig := parts[2]

	log.Printf("WebSocket connection request: ChainID=%d, Contract=%s, Event=%s", chainID, contractAddr, eventSig)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		server:   s,
		conn:     conn,
		send:     make(chan []byte, 256),
		chainID:  chainID,
		contract: contractAddr,
		event:    eventSig,
	}

	// Register client
	s.mu.Lock()
	clientID := fmt.Sprintf("%d-%s-%s", client.chainID, client.contract, client.event)
	s.clients[clientID] = client
	s.mu.Unlock()

	log.Printf("WebSocket client connected: %s", clientID)

	// Start client handlers
	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.server.mu.Lock()
		clientID := fmt.Sprintf("%d-%s-%s", c.chainID, c.contract, c.event)
		delete(c.server.clients, clientID)
		c.server.mu.Unlock()
		c.conn.Close()
		log.Printf("WebSocket client disconnected: %s", clientID)
	}()

	c.conn.SetReadLimit(c.server.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.server.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.server.config.PongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.server.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.server.config.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.server.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) BroadcastEvent(chainID int, contract, event string, data interface{}) error {
	message, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	clientID := fmt.Sprintf("%d-%s-%s", chainID, contract, event)

	if client, ok := s.clients[clientID]; ok {
		select {
		case client.send <- message:
			log.Printf("Sent event to client %s", clientID)
		default:
			log.Printf("Client %s is slow, message dropped", clientID)
		}
	} else {
		log.Printf("No clients found for %s", clientID)
	}

	return nil
}

func IsWebSocketUpgrade(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}
