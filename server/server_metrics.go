package server

import (
	"fmt"
	"os"

	"event-pool/network"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

// serverMetrics holds the metric instances of all sub systems
type serverMetrics struct {
	network *network.Metrics
}

// metricProvider serverMetric instance for the given ChainID and nameSpace
func metricProvider(nameSpace string, chainID string, metricsRequired bool) *serverMetrics {
	if metricsRequired {
		return &serverMetrics{
			network: network.GetPrometheusMetrics(nameSpace, "chain_id", chainID),
		}
	}

	return &serverMetrics{
		network: network.NilMetrics(),
	}
}

// enableDataDogProfiler enables DataDog profiler. Enable it by setting DD_ENABLE env var.
// Additional parameters can be set with env vars (DD_) - https://docs.datadoghq.com/profiler/enabling/go/
func (s *Server) enableDataDogProfiler() error {
	if os.Getenv("DD_PROFILING_ENABLED") == "" {
		s.logger.Debug("DataDog profiler disabled, set DD_PROFILING_ENABLED env var to enable it.")

		return nil
	}
	// For containerized solutions, we want to be able to set the ip and port that the agent will bind to
	// by defining DD_AGENT_HOST and DD_TRACE_AGENT_PORT env vars.
	// If these env vars are not defined, the agent will bind to default ip:port ( localhost:8126 )
	ddIP := "localhost"
	ddPort := "8126"

	if os.Getenv("DD_AGENT_HOST") != "" {
		ddIP = os.Getenv("DD_AGENT_HOST")
	}

	if os.Getenv("DD_TRACE_AGENT_PORT") != "" {
		ddPort = os.Getenv("DD_TRACE_AGENT_PORT")
	}

	if err := profiler.Start(
		// enable all profiles
		profiler.WithProfileTypes(
			profiler.CPUProfile,
			profiler.HeapProfile,
			profiler.BlockProfile,
			profiler.MutexProfile,
			profiler.GoroutineProfile,
			profiler.MetricsProfile,
		),
		profiler.WithAgentAddr(ddIP+":"+ddPort),
	); err != nil {
		return fmt.Errorf("could not start datadog profiler: %w", err)
	}

	// start the tracer
	tracer.Start()
	s.logger.Info("DataDog profiler started")

	return nil
}

func (s *Server) closeDataDogProfiler() {
	s.logger.Debug("closing DataDog profiler")
	profiler.Stop()

	s.logger.Debug("closing DataDog tracer")
	tracer.Stop()
}
