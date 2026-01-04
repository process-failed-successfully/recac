package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()

	// Verify all metrics are initialized
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDuration)
	assert.NotNil(t, m.MemoryUsage)
	assert.NotNil(t, m.CPUUsage)
	assert.NotNil(t, m.GoroutinesCount)
	assert.NotNil(t, m.JobsCompleted)
	assert.NotNil(t, m.JobsFailed)
	assert.NotNil(t, m.AgentStatus)
	assert.NotNil(t, m.TasksProcessed)
	assert.NotNil(t, m.TasksInProgress)
}

func TestRequestTrackingMiddleware(t *testing.T) {
	m := NewMetrics()

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	wrappedHandler := m.RequestTrackingMiddleware(handler)

	// Create test server
	ts := httptest.NewServer(wrappedHandler)
	defer ts.Close()

	// Make a request
	resp, err := http.Get(ts.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify metrics were recorded
	metric, err := prometheus.DefaultGatherer.Gather()
	assert.NoError(t, err)

	// Find the http_requests_total metric
	var found bool
	for _, mf := range metric {
		if *mf.Name == "http_requests_total" {
			found = true
			assert.Equal(t, 1.0, *mf.Metric[0].Counter.Value)
			break
		}
	}
	assert.True(t, found, "http_requests_total metric not found")
}

func TestUpdateSystemMetrics(t *testing.T) {
	m := NewMetrics()

	// Update system metrics
	m.UpdateSystemMetrics(2048, 0.5, 10)

	// Verify metrics were updated
	metric, err := prometheus.DefaultGatherer.Gather()
	assert.NoError(t, err)

	// Check memory usage
	var memoryFound, cpuFound, goroutinesFound bool
	for _, mf := range metric {
		if *mf.Name == "process_memory_bytes" {
			memoryFound = true
			assert.Equal(t, 2048.0, *mf.Metric[0].Gauge.Value)
		}
		if *mf.Name == "process_cpu_seconds_total" {
			cpuFound = true
			assert.Equal(t, 0.5, *mf.Metric[0].Gauge.Value)
		}
		if *mf.Name == "go_goroutines" {
			goroutinesFound = true
			assert.Equal(t, 10.0, *mf.Metric[0].Gauge.Value)
		}
	}

	assert.True(t, memoryFound, "process_memory_bytes metric not found")
	assert.True(t, cpuFound, "process_cpu_seconds_total metric not found")
	assert.True(t, goroutinesFound, "go_goroutines metric not found")
}

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics()

	// Create a request to the metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Serve the request
	m.Handler().ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", w.Header().Get("Content-Type"))

	// Verify response contains expected metrics
	response := w.Body.String()
	assert.Contains(t, response, "http_requests_total")
	assert.Contains(t, response, "http_request_duration_seconds")
	assert.Contains(t, response, "process_memory_bytes")
	assert.Contains(t, response, "process_cpu_seconds_total")
	assert.Contains(t, response, "go_goroutines")
	assert.Contains(t, response, "jobs_completed_total")
	assert.Contains(t, response, "jobs_failed_total")
	assert.Contains(t, response, "agent_status")
	assert.Contains(t, response, "tasks_processed_total")
	assert.Contains(t, response, "tasks_in_progress")
}

func TestCustomBusinessMetrics(t *testing.T) {
	m := NewMetrics()

	// Increment custom metrics
	m.JobsCompleted.WithLabelValues("build", "success").Inc()
	m.JobsFailed.WithLabelValues("test", "timeout").Inc()
	m.AgentStatus.WithLabelValues("agent-1", "builder").Set(1)
	m.TasksProcessed.Inc()
	m.TasksInProgress.Inc()

	// Verify metrics were recorded
	metric, err := prometheus.DefaultGatherer.Gather()
	assert.NoError(t, err)

	// Check jobs_completed_total
	var jobsCompletedFound, jobsFailedFound, agentStatusFound, tasksProcessedFound, tasksInProgressFound bool
	for _, mf := range metric {
		if *mf.Name == "jobs_completed_total" {
			jobsCompletedFound = true
			assert.Equal(t, 1.0, *mf.Metric[0].Counter.Value)
		}
		if *mf.Name == "jobs_failed_total" {
			jobsFailedFound = true
			assert.Equal(t, 1.0, *mf.Metric[0].Counter.Value)
		}
		if *mf.Name == "agent_status" {
			agentStatusFound = true
			assert.Equal(t, 1.0, *mf.Metric[0].Gauge.Value)
		}
		if *mf.Name == "tasks_processed_total" {
			tasksProcessedFound = true
			assert.Equal(t, 1.0, *mf.Metric[0].Counter.Value)
		}
		if *mf.Name == "tasks_in_progress" {
			tasksInProgressFound = true
			assert.Equal(t, 1.0, *mf.Metric[0].Gauge.Value)
		}
	}

	assert.True(t, jobsCompletedFound, "jobs_completed_total metric not found")
	assert.True(t, jobsFailedFound, "jobs_failed_total metric not found")
	assert.True(t, agentStatusFound, "agent_status metric not found")
	assert.True(t, tasksProcessedFound, "tasks_processed_total metric not found")
	assert.True(t, tasksInProgressFound, "tasks_in_progress metric not found")
}
