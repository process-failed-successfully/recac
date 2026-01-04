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

func TestMetricsInitialization(t *testing.T) {
	m := NewMetrics()

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
	assert.NotNil(t, m.TaskProcessingTime)
}

func TestMiddleware(t *testing.T) {
	m := NewMetrics()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	// Verify metrics were recorded
	metric, err := m.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/test", "OK")
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.GetCounter().GetValue())
}

func TestCustomBusinessMetrics(t *testing.T) {
	m := NewMetrics()

	// Test JobsCompleted
	m.JobsCompleted.WithLabelValues("build", "success").Inc()
	metric, err := m.JobsCompleted.GetMetricWithLabelValues("build", "success")
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.GetCounter().GetValue())

	// Test JobsFailed
	m.JobsFailed.WithLabelValues("build", "timeout").Inc()
	metric, err = m.JobsFailed.GetMetricWithLabelValues("build", "timeout")
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.GetCounter().GetValue())

	// Test AgentStatus
	m.AgentStatus.WithLabelValues("agent-1", "builder").Set(1)
	metric, err = m.AgentStatus.GetMetricWithLabelValues("agent-1", "builder")
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metric.GetGauge().GetValue())

	// Test TasksProcessed
	m.TasksProcessed.Inc()
	assert.Equal(t, float64(1), m.TasksProcessed.GetCounter().GetValue())

	// Test TasksInProgress
	m.TasksInProgress.Inc()
	assert.Equal(t, float64(1), m.TasksInProgress.GetGauge().GetValue())

	// Test TaskProcessingTime
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	m.TaskProcessingTime.WithLabelValues("build").Observe(time.Since(start).Seconds())
	metric, err = m.TaskProcessingTime.GetMetricWithLabelValues("build")
	assert.NoError(t, err)
	assert.True(t, metric.GetHistogram().GetSampleCount() > 0)
}

func TestMetricsEndpoint(t *testing.T) {
	m := NewMetrics()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create a test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request
	resp, err := http.Get(server.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify metrics endpoint
	metricsHandler := promhttp.Handler()
	metricsServer := httptest.NewServer(metricsHandler)
	defer metricsServer.Close()

	metricsResp, err := http.Get(metricsServer.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, metricsResp.StatusCode)
	assert.Contains(t, metricsResp.Header.Get("Content-Type"), "text/plain")
}
