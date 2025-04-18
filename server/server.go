package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"event-pool/chain"
	db2 "event-pool/internal/db"
	"event-pool/network"
	"event-pool/prisma/db"
	"event-pool/secrets"
	"event-pool/server/proto"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/grpclb/state"
	"google.golang.org/grpc/credentials/insecure"
)

// Server is the central manager of the blockchain client
type Server struct {
	logger *zap.SugaredLogger
	config *Config

	//consensus consensus.Consensus

	chain *chain.NodeChain

	// state executor
	//executor *state.Executor

	// system grpc server
	grpcServer *grpc.Server

	// libp2p network
	network *network.Server

	serverMetrics *serverMetrics

	prometheusServer *http.Server

	// secrets manager
	secretsManager secrets.SecretsManager

	db *db.PrismaClient
}

var dirPaths = []string{
	"blockchain",
	"trie",
}

// newCLILogger returns minimal logger instance that sends all logs to standard output
func newCLILogger(config *Config) *zap.SugaredLogger {
	if config.LogLevel == zap.DebugLevel {
		return zap.NewExample().Sugar()
	}
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

// newLoggerFromConfig creates a new logger which logs to a specified file.
// If log file is not set it outputs to standard output ( console ).
// If log file is specified, and it can't be created the server command will error out
func newLoggerFromConfig(config *Config) (*zap.SugaredLogger, error) {
	return newCLILogger(config), nil
}

// NewServer creates a new Minimal server, using the passed in configuration
func NewServer(config *Config) (*Server, error) {
	logger, err := newLoggerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not setup new logger instance, %w", err)
	}

	dbClient, err := db2.NewClient(db.WithDatasourceURL(config.DbUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	m := &Server{
		logger:     logger.Named("server"),
		config:     config,
		chain:      config.Chain,
		grpcServer: grpc.NewServer(grpc.UnaryInterceptor(loggerInterceptor(logger.Named("grpc_server")))),
		db:         dbClient,
	}

	m.logger.Infow("Data dir", "path", config.DataDir)
	m.logger.Infow("Config", "config", config)

	//// Generate all the paths in the dataDir
	//if err := common.SetupDataDir(config.DataDir, dirPaths); err != nil {
	//	return nil, fmt.Errorf("failed to create data directories: %w", err)
	//}

	if config.Telemetry.PrometheusAddr != nil {
		m.serverMetrics = metricProvider("event-pool", config.Chain.Name, true)
		m.prometheusServer = m.startPrometheusServer(config.Telemetry.PrometheusAddr)
	} else {
		m.serverMetrics = metricProvider("event-pool", config.Chain.Name, false)
	}

	// Set up datadog profiler
	if ddErr := m.enableDataDogProfiler(); ddErr != nil {
		m.logger.Error("DataDog profiler setup failed", "err", ddErr.Error())
	}

	// Set up the secrets manager
	if err := m.setupSecretsManager(); err != nil {
		return nil, fmt.Errorf("failed to set up the secrets manager: %w", err)
	}

	// start libp2p
	{
		netConfig := config.Network
		netConfig.Chain = m.config.Chain
		netConfig.DataDir = filepath.Join(m.config.DataDir, "libp2p")
		netConfig.SecretsManager = m.secretsManager
		netConfig.Metrics = m.serverMetrics.network

		networkSvr, err := network.NewServer(logger, netConfig)
		if err != nil {
			return nil, err
		}
		m.network = networkSvr
	}

	// setup and start grpc server
	if err := m.setupHTTP(); err != nil {
		return nil, err
	}

	if err := m.setupGRPC(); err != nil {
		return nil, err
	}

	if err := m.network.Start(); err != nil {
		return nil, err
	}

	return m, nil
}

// setupSecretsManager sets up the secrets manager
func (s *Server) setupSecretsManager() error {
	secretsManagerConfig := s.config.SecretsManager
	if secretsManagerConfig == nil {
		// No config provided, use default
		secretsManagerConfig = &secrets.SecretsManagerConfig{
			Type: secrets.Local,
		}
	}

	secretsManagerType := secretsManagerConfig.Type
	secretsManagerParams := &secrets.SecretsManagerParams{
		Logger: s.logger,
	}

	if secretsManagerType == secrets.Local {
		// Only the base directory is required for
		// the local secrets manager
		secretsManagerParams.Extra = map[string]interface{}{
			secrets.Path: s.config.DataDir,
		}
	}

	// Grab the factory method
	secretsManagerFactory, ok := secretsManagerBackends[secretsManagerType]
	if !ok {
		return fmt.Errorf("secrets manager type '%s' not found", secretsManagerType)
	}

	// Instantiate the secrets manager
	secretsManager, factoryErr := secretsManagerFactory(
		secretsManagerConfig,
		secretsManagerParams,
	)

	if factoryErr != nil {
		return fmt.Errorf("unable to instantiate secrets manager, %w", factoryErr)
	}

	s.secretsManager = secretsManager

	return nil
}

type jsonRPCHub struct {
	state state.State

	*txpool.TxPool
	*network.Server
}

// HELPER + WRAPPER METHODS //

func (j *jsonRPCHub) GetPeers() int {
	return len(j.Server.Peers())
}

// SETUP //

// setupHTTP sets up the http server and listens on tcp
func (s *Server) setupHTTP() error {
	s.logger.Infow("http server started", "addr", s.config.JSONRPC.JSONRPCAddr.String())
	lis, err := net.Listen("tcp", s.config.JSONRPC.JSONRPCAddr.String())
	if err != nil {
		return err
	}

	//mux := http.NewServeMux()
	gwMux := runtime.NewServeMux()

	// Register your gRPC Gateway handler (System service)
	err = proto.RegisterSystemHandlerFromEndpoint(
		context.Background(),
		gwMux,
		s.config.GRPCAddr.String(),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		return err
	}

	// Optionally wrap with custom routes like /health
	httpMux := http.NewServeMux()
	httpMux.Handle("/", gwMux)

	srv := &http.Server{
		Handler:           httpMux,
		ReadHeaderTimeout: 60 * time.Second,
	}

	go func() {
		if err = srv.Serve(lis); err != nil {
			s.logger.Errorw("closed http connection", "err", err)
		}
	}()

	return nil
}

// setupGRPC sets up the grpc server and listens on tcp
func (s *Server) setupGRPC() error {
	proto.RegisterSystemServer(s.grpcServer, &systemService{server: s})

	lis, err := net.Listen("tcp", s.config.GRPCAddr.String())
	if err != nil {
		return err
	}

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error(err.Error())
		}
	}()

	s.logger.Infow("GRPC server running", "addr", s.config.GRPCAddr.String())

	return nil
}

// Chain returns the chain object of the client
func (s *Server) Chain() *chain.NodeChain {
	return s.chain
}

// JoinPeer attempts to add a new peer to the networking server
func (s *Server) JoinPeer(rawPeerMultiaddr string) error {
	return s.network.JoinPeer(rawPeerMultiaddr)
}

// Close closes the Minimal server (blockchain, networking, consensus)
func (s *Server) Close() {
	// Close the networking layer
	if err := s.network.Close(); err != nil {
		s.logger.Error("failed to close networking", "err", err.Error())
	}

	if s.prometheusServer != nil {
		if err := s.prometheusServer.Shutdown(context.Background()); err != nil {
			s.logger.Error("Prometheus server shutdown error", err)
		}
	}

	// close DataDog profiler
	s.closeDataDogProfiler()
	db2.Close(s.db)
}

// Entry is a consensus configuration entry
type Entry struct {
	Enabled bool
	Config  map[string]interface{}
}

func (s *Server) startPrometheusServer(listenAddr *net.TCPAddr) *http.Server {
	srv := &http.Server{
		Addr: listenAddr.String(),
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{},
			),
		),
		ReadHeaderTimeout: 60 * time.Second,
	}

	go func() {
		s.logger.Infow("Prometheus server started", "addr=", listenAddr.String())

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
		}
	}()

	return srv
}

func loggerInterceptor(logger *zap.SugaredLogger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)
		duration := time.Since(start)

		logger.Infow("gRPC call",
			"method", info.FullMethod,
			"duration", duration,
			"error", err,
		)

		return resp, err
	}
}
