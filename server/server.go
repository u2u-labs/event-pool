package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"event-pool/blockchain"
	"event-pool/chain"
	"event-pool/consensus"
	"event-pool/consensus/ibft"
	"event-pool/crypto"
	"event-pool/helper/common"
	configHelper "event-pool/helper/config"
	"event-pool/helper/keccak"
	db2 "event-pool/internal/db"
	"event-pool/network"
	"event-pool/prisma/db"
	"event-pool/secrets"
	"event-pool/server/proto"
	"event-pool/state"
	itrie "event-pool/state/immutable-trie"
	"event-pool/txpool"
	proto2 "event-pool/txpool/proto"
	"event-pool/types"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server is the central manager of the blockchain client
type Server struct {
	logger       *zap.SugaredLogger
	config       *Config
	state        state.State
	stateStorage itrie.Storage

	consensus consensus.Consensus

	blockchain *blockchain.Blockchain
	chain      *chain.NodeChain

	// state executor
	executor *state.Executor

	// system grpc server
	grpcServer *grpc.Server

	// libp2p network
	network *network.Server

	// transaction pool
	txpool *txpool.TxPool

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
	// Create a config suitable for CLI usage
	zapConfig := zap.NewDevelopmentConfig()

	// Apply the desired log level
	zapConfig.Level = zap.NewAtomicLevelAt(config.LogLevel)

	// Optionally change encoding to "console" for better CLI readability
	zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := zapConfig.Build()
	if err != nil {
		panic(err) // or handle gracefully
	}

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

	// Generate all the paths in the dataDir
	if err := common.SetupDataDir(config.DataDir, dirPaths); err != nil {
		return nil, fmt.Errorf("failed to create data directories: %w", err)
	}

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

	// start blockchain object
	//stateStorage, err := itrie.NewPrismaStorage(m.db, logger.Named("trie"))
	stateStorage, err := itrie.NewLevelDBStorage(filepath.Join(m.config.DataDir, "trie"), logger)
	if err != nil {
		return nil, err
	}

	m.stateStorage = stateStorage

	st := itrie.NewState(stateStorage)
	m.state = st

	m.executor = state.NewExecutor(config.Chain.Params, st, logger)

	// use the eip155 signer
	signer := crypto.NewEIP155Signer(uint64(m.config.Chain.Params.ChainID))

	cfg := config.Chain.Clone()
	cfg.NodeStorageAddress = types.StringToAddress(config.NodeStorageAddress)
	cfg.RpcInfo = &chain.RpcInfo{}
	*cfg.RpcInfo = m.config.EthereumRpc.Chains[m.config.Chain.Params.ChainID]
	cfg.Genesis.ChainId = uint64(m.config.Chain.Params.ChainID)
	// blockchain object
	m.blockchain, err = blockchain.NewBlockchain(logger, m.config.DataDir, cfg, nil, m.executor, signer)
	if err != nil {
		return nil, err
	}

	{
		hub := &txpoolHub{
			Blockchain: m.blockchain,
			state:      m.state,
		}

		deploymentWhitelist, err := configHelper.GetDeploymentWhitelist(config.Chain)
		if err != nil {
			return nil, err
		}

		// start transaction pool
		m.txpool, err = txpool.NewTxPool(
			logger.Named("txpool"),
			hub,
			m.grpcServer,
			m.network,
			m.serverMetrics.txpool,
			&txpool.Config{
				MaxSlots:            m.config.MaxSlots,
				PriceLimit:          m.config.PriceLimit,
				MaxAccountEnqueued:  m.config.MaxAccountEnqueued,
				DeploymentWhitelist: deploymentWhitelist,
			},
		)
		if err != nil {
			return nil, err
		}

		m.txpool.SetSigner(signer)
	}

	m.executor.SetChainId(uint64(m.config.Chain.Params.ChainID))
	m.executor.FnGetRpcClient = m.blockchain.GetEthereumClient
	m.executor.FnGetMonitor = m.blockchain.GetMonitor

	{
		// Setup consensus
		if err := m.setupConsensus(); err != nil {
			return nil, err
		}
		m.blockchain.SetConsensus(m.consensus)
	}

	// after consensus is done, we can mine the genesis block in blockchain
	// This is done because consensus might use a custom Hash function so we need
	// to wait for consensus because we do any block hashing like genesis
	if err := m.blockchain.ComputeGenesis(); err != nil {
		return nil, err
	}

	// initialize data in consensus layer
	if err := m.consensus.Initialize(); err != nil {
		return nil, err
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

	// start consensus
	if err := m.consensus.Start(); err != nil {
		return nil, err
	}

	m.txpool.Start()

	return m, nil
}

// setupConsensus sets up the consensus mechanism
func (s *Server) setupConsensus() error {
	config := &consensus.Config{
		Params: s.config.Chain.Params,
		Logger: s.logger.Named("consensus"),
		Config: map[string]any{"type": "PoS"},
	}

	consensus, err := ibft.Factory(
		&consensus.Params{
			Context:        context.Background(),
			Config:         config,
			Network:        s.network,
			Blockchain:     s.blockchain,
			Executor:       s.executor,
			TxPool:         s.txpool,
			Grpc:           s.grpcServer,
			Logger:         s.logger,
			Metrics:        s.serverMetrics.consensus,
			SecretsManager: s.secretsManager,
			BlockTime:      s.config.BlockTime,
		},
	)

	if err != nil {
		return err
	}

	s.consensus = consensus

	return nil
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

// HELPER + WRAPPER METHODS //

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
	if err = proto2.RegisterTxnPoolOperatorHandlerFromEndpoint(
		context.Background(),
		gwMux,
		s.config.GRPCAddr.String(),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}); err != nil {
		return err
	}

	// Optionally wrap with custom routes like /health
	httpMux := http.NewServeMux()
	httpMux.Handle("/", gwMux)
	httpMux.HandleFunc("/ws", s.txpool.HandleWs)

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

type txpoolHub struct {
	state state.State
	*blockchain.Blockchain
}

func (t *txpoolHub) GetNonce(root types.Hash, addr types.Address) uint64 {
	snap, err := t.state.NewSnapshotAt(root)
	if err != nil {
		return 0
	}

	result, ok := snap.Get(keccak.Keccak256(nil, addr.Bytes()))
	if !ok {
		return 0
	}

	var account state.Account

	if err := account.UnmarshalRlp(result); err != nil {
		return 0
	}

	return account.Nonce
}

func (t *txpoolHub) GetBalance(root types.Hash, addr types.Address) (*big.Int, error) {
	snap, err := t.state.NewSnapshotAt(root)
	if err != nil {
		return nil, fmt.Errorf("unable to get snapshot for root, %w", err)
	}

	result, ok := snap.Get(keccak.Keccak256(nil, addr.Bytes()))
	if !ok {
		return big.NewInt(0), nil
	}

	var account state.Account
	if err = account.UnmarshalRlp(result); err != nil {
		return nil, fmt.Errorf("unable to unmarshal account from snapshot, %w", err)
	}

	return account.Balance, nil
}
