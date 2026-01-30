package telemetry

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics Definitions
var (
	// 1. Code Generation
	LinesGeneratedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_lines_generated_total",
		Help: "Total lines of code written by agents.",
	}, []string{"project"})
	FilesCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_files_created_total",
		Help: "Total new files created.",
	}, []string{"project"})
	FilesModifiedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_files_modified_total",
		Help: "Total files modified.",
	}, []string{"project"})
	BuildSuccessTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_build_success_total",
		Help: "Number of successful builds.",
	}, []string{"project"})
	BuildFailureTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_build_failure_total",
		Help: "Number of failed builds.",
	}, []string{"project"})

	// 2. Agent Performance
	AgentIterationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_agent_iterations_total",
		Help: "Total agent turns/iterations.",
	}, []string{"project"})
	AgentResponseTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "recac_agent_response_time_seconds",
		Help:    "Latency of LLM responses.",
		Buckets: prometheus.DefBuckets,
	}, []string{"project"})
	TokenUsageTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_token_usage_total",
		Help: "Total tokens used (prompt + completion).",
	}, []string{"project"})
	AgentStallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_agent_stalls_total",
		Help: "Number of times agents stalled/made no progress.",
	}, []string{"project"})
	ContextWindowUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recac_context_window_usage",
		Help: "Current percentage of context window usage.",
	}, []string{"project"})

	// 3. Multi-Agent Orchestration
	ActiveAgents = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recac_active_agents",
		Help: "Number of currently running agent workers.",
	}, []string{"project"})
	TasksPending = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recac_tasks_pending",
		Help: "Number of tasks in the queue.",
	}, []string{"project"})
	TasksCompletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_tasks_completed_total",
		Help: "Total completed tasks.",
	}, []string{"project"})
	LockContentionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_lock_contention_total",
		Help: "Number of times agents failed to acquire a file lock.",
	}, []string{"project"})
	OrchestratorLoopsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_orchestrator_loops_total",
		Help: "Number of scheduling cycles.",
	}, []string{"project"})

	// 4. System Reliability
	ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_errors_total",
		Help: "Total internal errors by type.",
	}, []string{"project", "type"})
	DBOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_db_operations_total",
		Help: "Total database reads/writes.",
	}, []string{"project"})
	DockerOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_docker_ops_total",
		Help: "Total Docker command executions.",
	}, []string{"project"})
	DockerErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recac_docker_errors_total",
		Help: "Docker execution failures.",
	}, []string{"project"})
	UptimeSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recac_uptime_seconds",
		Help: "Session duration in seconds.",
	}, []string{"project"})
)

var (
	metricsOnce    sync.Once
	metricsMu      sync.Mutex
	metricsRunning bool
)

// StartMetricsServer starts a HTTP server exposing Prometheus metrics.
// It attempts to bind to the given port. If the port is in use, it will
// try the next 10 ports before giving up.
func StartMetricsServer(basePort int) error {
	metricsMu.Lock()
	if metricsRunning {
		metricsMu.Unlock()
		return nil // Already running
	}
	metricsRunning = true
	metricsMu.Unlock()

	metricsOnce.Do(func() {
		http.Handle("/metrics", promhttp.Handler())
	})

	var listener net.Listener
	var err error

	// Try up to 10 ports
	for i := 0; i < 10; i++ {
		port := basePort + i
		addr := ":" + strconv.Itoa(port)
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			fmt.Fprintf(os.Stderr, "Starting metrics server on %s\n", addr)
			return http.Serve(listener, nil)
		}
	}

	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()
	return fmt.Errorf("failed to find available port starting from %d: %w", basePort, err)
}

// API Helper Functions

func TrackLineGenerated(project string, count int) {
	LinesGeneratedTotal.WithLabelValues(project).Add(float64(count))
}

func TrackFileCreated(project string) {
	FilesCreatedTotal.WithLabelValues(project).Inc()
}

func TrackFileModified(project string) {
	FilesModifiedTotal.WithLabelValues(project).Inc()
}

func TrackBuildResult(project string, success bool) {
	if success {
		BuildSuccessTotal.WithLabelValues(project).Inc()
	} else {
		BuildFailureTotal.WithLabelValues(project).Inc()
	}
}

func TrackAgentIteration(project string) {
	AgentIterationsTotal.WithLabelValues(project).Inc()
}

func ObserveAgentLatency(project string, seconds float64) {
	AgentResponseTime.WithLabelValues(project).Observe(seconds)
}

func TrackTokenUsage(project string, count int) {
	TokenUsageTotal.WithLabelValues(project).Add(float64(count))
}

func TrackAgentStall(project string) {
	AgentStallsTotal.WithLabelValues(project).Inc()
}

func SetContextUsage(project string, percent float64) {
	ContextWindowUsage.WithLabelValues(project).Set(percent)
}

func SetActiveAgents(project string, count int) {
	ActiveAgents.WithLabelValues(project).Set(float64(count))
}

func SetTasksPending(project string, count int) {
	TasksPending.WithLabelValues(project).Set(float64(count))
}

func TrackTaskCompleted(project string) {
	TasksCompletedTotal.WithLabelValues(project).Inc()
}

func TrackLockContention(project string) {
	LockContentionTotal.WithLabelValues(project).Inc()
}

func TrackOrchestratorLoop(project string) {
	OrchestratorLoopsTotal.WithLabelValues(project).Inc()
}

func TrackError(project string, errType string) {
	ErrorsTotal.WithLabelValues(project, errType).Inc()
}

func TrackDBOp(project string) {
	DBOperationsTotal.WithLabelValues(project).Inc()
}

func TrackDockerOp(project string) {
	DockerOpsTotal.WithLabelValues(project).Inc()
}

func TrackDockerError(project string) {
	DockerErrorsTotal.WithLabelValues(project).Inc()
}
