package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type serverType int

const (
	serverIPC serverType = iota
	serverHTTP
	serverWS
)

func (s serverType) String() string {
	switch s {
	case serverIPC:
		return "ipc"
	case serverHTTP:
		return "http"
	case serverWS:
		return "ws"
	default:
		panic("BUG: Not expected")
	}
}

// JSONRPC is an API consensus
type JSONRPC struct {
	logger     *zap.SugaredLogger
	config     *Config
	dispatcher dispatcher
}

type dispatcher interface {
}

// JSONRPCStore defines all the methods required
// by all the JSON RPC endpoints
type JSONRPCStore interface {
	networkStore
}

type Config struct {
	Store                    JSONRPCStore
	Addr                     *net.TCPAddr
	ChainID                  uint64
	ChainName                string
	AccessControlAllowOrigin []string
	PriceLimit               uint64
	BatchLengthLimit         uint64
	BlockRangeLimit          uint64
}

// NewJSONRPC returns the JSONRPC http server
func NewJSONRPC(logger *zap.SugaredLogger, config *Config) (*JSONRPC, error) {
	srv := &JSONRPC{
		logger: logger.Named("jsonrpc"),
		config: config,
		dispatcher: newDispatcher(
			logger,
			config.Store,
			&dispatcherParams{
				chainID:                 config.ChainID,
				chainName:               config.ChainName,
				priceLimit:              config.PriceLimit,
				jsonRPCBatchLengthLimit: config.BatchLengthLimit,
				blockRangeLimit:         config.BlockRangeLimit,
			},
		),
	}

	// start http server
	if err := srv.setupHTTP(); err != nil {
		return nil, err
	}

	return srv, nil
}

func (j *JSONRPC) setupHTTP() error {
	j.logger.Info("http server started", "addr", j.config.Addr.String())

	lis, err := net.Listen("tcp", j.config.Addr.String())
	if err != nil {
		return err
	}

	// NewServeMux must be used, as it disables all debug features.
	// For some strange reason, with DefaultServeMux debug/vars is always enabled (but not debug/pprof).
	// If pprof need to be enabled, this should be DefaultServeMux
	mux := http.NewServeMux()

	// The middleware factory returns a handler, so we need to wrap the handler function properly.
	jsonRPCHandler := http.HandlerFunc(j.handle)
	mux.Handle("/", middlewareFactory(j.config)(jsonRPCHandler))

	//mux.HandleFunc("/ws", j.handleWs)

	srv := http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 60 * time.Second,
	}

	go func() {
		if err := srv.Serve(lis); err != nil {
			j.logger.Error("closed http connection", "err", err)
		}
	}()

	return nil
}

// The middlewareFactory builds a middleware which enables CORS using the provided config.
func middlewareFactory(config *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			for _, allowedOrigin := range config.AccessControlAllowOrigin {
				if allowedOrigin == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")

					break
				}

				if allowedOrigin == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)

					break
				}
			}
			next.ServeHTTP(w, r)
		})
	}
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

// wsWrapper is a wrapping object for the web socket connection and logger
type wsWrapper struct {
	sync.Mutex

	ws       *websocket.Conn    // the actual WS connection
	logger   *zap.SugaredLogger // module logger
	filterID string             // filter ID
}

func (w *wsWrapper) SetFilterID(filterID string) {
	w.filterID = filterID
}

func (w *wsWrapper) GetFilterID() string {
	return w.filterID
}

// WriteMessage writes out the message to the WS peer
func (w *wsWrapper) WriteMessage(messageType int, data []byte) error {
	w.Lock()
	defer w.Unlock()
	writeErr := w.ws.WriteMessage(messageType, data)

	if writeErr != nil {
		w.logger.Error(
			fmt.Sprintf("Unable to write WS message, %s", writeErr.Error()),
		)
	}

	return writeErr
}

// isSupportedWSType returns a status indicating if the message type is supported
func isSupportedWSType(messageType int) bool {
	return messageType == websocket.TextMessage ||
		messageType == websocket.BinaryMessage
}

func (j *JSONRPC) handle(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set(
		"Access-Control-Allow-Headers",
		"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
	)

	switch req.Method {
	case "POST":
		j.handleJSONRPCRequest(w, req)
	case "GET":
		j.handleGetRequest(w)
	case "OPTIONS":
		// nothing to return
	default:
		_, _ = w.Write([]byte("method " + req.Method + " not allowed"))
	}
}

func (j *JSONRPC) handleJSONRPCRequest(w http.ResponseWriter, req *http.Request) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))

		return
	}

	// log request
	j.logger.Debug("handle", "request", string(data))

	//resp, err := j.dispatcher.Handle(data)
	//
	//if err != nil {
	//	_, _ = w.Write([]byte(err.Error()))
	//} else {
	//	_, _ = w.Write(resp)
	//}
	//
	//j.logger.Debug("handle", "response", string(resp))
}

type GetResponse struct {
	Name    string `json:"name"`
	ChainID uint64 `json:"chain_id"`
	Version string `json:"version"`
}

func (j *JSONRPC) handleGetRequest(writer io.Writer) {
	data := &GetResponse{
		Name:    j.config.ChainName,
		ChainID: j.config.ChainID,
		Version: "0.1.0",
	}

	resp, err := json.Marshal(data)
	if err != nil {
		_, _ = writer.Write([]byte(err.Error()))
	}

	if _, err = writer.Write(resp); err != nil {
		_, _ = writer.Write([]byte(err.Error()))
	}
}
