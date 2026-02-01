package load

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTester_Run_RequestLimit(t *testing.T) {
	var serverCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&serverCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		URL:         server.URL,
		Requests:    50,
		Concurrency: 5,
	}

	tester := NewTester(cfg)
	stats := tester.Run(context.Background())

	assert.Equal(t, 50, stats.TotalRequests)
	assert.Equal(t, 50, stats.Success)
	assert.Equal(t, 0, stats.Failures)
	assert.Equal(t, int64(50), serverCount)
}

func TestTester_Run_DurationLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		URL:         server.URL,
		Duration:    100 * time.Millisecond,
		Concurrency: 2,
	}

	tester := NewTester(cfg)
	start := time.Now()
	stats := tester.Run(context.Background())
	elapsed := time.Since(start)

	// Should be roughly 100ms. Allow significant buffer for scheduler/overhead.
	// CI environments can be slow.
	assert.InDelta(t, 100*time.Millisecond, elapsed, float64(200*time.Millisecond))
	assert.Greater(t, stats.TotalRequests, 0)
}

func TestTester_Run_Headers(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Test-Header")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		URL:         server.URL,
		Requests:    1,
		Headers:     []string{"X-Test-Header: MyValue"},
		Concurrency: 1,
	}

	tester := NewTester(cfg)
	tester.Run(context.Background())

	assert.Equal(t, "MyValue", receivedHeader)
}

func TestCalculateStats(t *testing.T) {
	results := []Result{
		{Duration: 10 * time.Millisecond, Status: 200},
		{Duration: 20 * time.Millisecond, Status: 200},
		{Duration: 30 * time.Millisecond, Status: 500},
		{Duration: 40 * time.Millisecond, Status: 200},
		{Duration: 50 * time.Millisecond, Status: 200},
	}

	// Sorted: 10, 20, 30, 40, 50
	// Avg: 30
	// P50: 30 (index 2)
	// P90: 50 (index 4)

	stats := calculateStats(results, 1*time.Second)

	assert.Equal(t, 5, stats.TotalRequests)
	assert.Equal(t, 4, stats.Success)
	assert.Equal(t, 1, stats.Failures)
	assert.Equal(t, 5.0, stats.RPS) // 5 reqs / 1 sec
	assert.Equal(t, 30*time.Millisecond, stats.AvgLatency)
	assert.Equal(t, 10*time.Millisecond, stats.MinLatency)
	assert.Equal(t, 50*time.Millisecond, stats.MaxLatency)
	assert.Equal(t, 30*time.Millisecond, stats.P50Latency)
	assert.Equal(t, 50*time.Millisecond, stats.P90Latency)
}
