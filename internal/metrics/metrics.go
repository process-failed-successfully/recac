package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics represents the collection of all Prometheus metrics
type Metrics struct {
	// Standard metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	MemoryUsage         prometheus.Gauge
	CPUUsage            prometheus.Gauge
	GoroutinesCount     prometheus.Gauge

	// Custom business metrics
	JobsCompleted   *prometheus.CounterVec
	JobsFailed      *prometheus.CounterVec
	AgentStatus     *prometheus.GaugeVec
	TasksProcessed  prometheus.Counter
	TasksInProgress prometheus.Gauge
}

// NewMetrics creates and registers all standard and custom metrics
func NewMetrics() *Metrics {
	m := &Metrics{}

	// Standard HTTP metrics
	m.HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// System metrics
	m.MemoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_memory_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	m.CPUUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_cpu_seconds_total",
			Help: "Total CPU usage in seconds",
		},
	)

	m.GoroutinesCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of active goroutines",
		},
	)

	// Custom business metrics
	m.JobsCompleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_completed_total",
			Help: "Total number of jobs completed",
		},
		[]string{"job_type", "status"},
	)

	m.JobsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_failed_total",
			Help: "Total number of jobs failed",
		},
		[]string{"job_type", "reason"},
	)

	m.AgentStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_status",
			Help: "Current status of agents (1=active, 0=inactive)",
		},
		[]string{"agent_id", "agent_type"},
	)

	m.TasksProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tasks_processed_total",
			Help: "Total number of tasks processed",
		},
	)

	m.TasksInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tasks_in_progress",
			Help: "Number of tasks currently in progress",
		},
	)

	// Register all metrics
	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.MemoryUsage,
		m.CPUUsage,
		m.GoroutinesCount,
		m.JobsCompleted,
		m.JobsFailed,
		m.AgentStatus,
		m.TasksProcessed,
		m.TasksInProgress,
	)

	return m
}

// Middleware for tracking HTTP requests
func (m *Metrics) RequestTrackingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		// Record metrics
		m.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.statusCode)).Inc()
		m.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})
}

// responseWriter is a wrapper to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(memoryBytes uint64, cpuSeconds float64, goroutines int) {
	m.MemoryUsage.Set(float64(memoryBytes))
	m.CPUUsage.Add(cpuSeconds)
	m.GoroutinesCount.Set(float64(goroutines))
}

// Handler returns the Prometheus HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
