package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadTest_Requests(t *testing.T) {
	// Setup Server
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Setup Config
	// Save original globals
	origRequests := ltRequests
	origConcurrency := ltConcurrency
	origDuration := ltDuration
	origAnalyze := ltAnalyze
	defer func() {
		ltRequests = origRequests
		ltConcurrency = origConcurrency
		ltDuration = origDuration
		ltAnalyze = origAnalyze
	}()

	ltRequests = 100
	ltConcurrency = 10
	ltDuration = 0    // Request based
	ltAnalyze = false // Don't call agent

	// Run Test
	res, err := executeLoadTest(server.URL)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, 100, res.TotalRequests)
	assert.Equal(t, 100, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
	assert.Equal(t, int64(100), atomic.LoadInt64(&requestCount))
	assert.Len(t, res.Latencies, 100)
}

func TestLoadTest_Duration(t *testing.T) {
	// Setup Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		time.Sleep(10 * time.Millisecond) // Simulate work
	}))
	defer server.Close()

	// Setup Config
	origRequests := ltRequests
	origConcurrency := ltConcurrency
	origDuration := ltDuration
	defer func() {
		ltRequests = origRequests
		ltConcurrency = origConcurrency
		ltDuration = origDuration
	}()

	ltRequests = 0 // Duration based
	ltConcurrency = 5
	ltDuration = 100 * time.Millisecond

	// Run Test
	res, err := executeLoadTest(server.URL)

	// Assertions
	assert.NoError(t, err)
	assert.True(t, res.TotalDuration >= 100*time.Millisecond)
	// With 5 workers doing 10ms tasks for 100ms, expected ~50 requests
	// Allow for overhead, check > 0
	assert.Greater(t, res.TotalRequests, 0)
}

func TestLoadTest_Errors(t *testing.T) {
	// Setup Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Setup Config
	origRequests := ltRequests
	origConcurrency := ltConcurrency
	defer func() {
		ltRequests = origRequests
		ltConcurrency = origConcurrency
	}()

	ltRequests = 10
	ltConcurrency = 2

	// Run Test
	res, err := executeLoadTest(server.URL)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, 10, res.TotalRequests)
	// In our logic, 500s are counted as SuccessCount (HTTP wise) but we separate them in ErrorCount logic?
	// Let's check executeLoadTest logic:
	// res.StatusCodes[500]++
	// res.SuccessCount counts 2xx.
	// res.ErrorCount counts != 2xx.
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 10, res.ErrorCount)
	assert.Equal(t, 10, res.StatusCodes[500])
}

func TestLoadTest_InvalidURL(t *testing.T) {
	// Setup Config
	origRequests := ltRequests
	origConcurrency := ltConcurrency
	defer func() {
		ltRequests = origRequests
		ltConcurrency = origConcurrency
	}()
	ltRequests = 1
	ltConcurrency = 1

	// Run Test with invalid URL (connection refused)
	res, err := executeLoadTest("http://localhost:1") // Port 1 usually closed

	// Assertions
	// It should not error out completely, but record network errors
	assert.NoError(t, err)
	assert.Equal(t, 1, res.TotalRequests)
	assert.Equal(t, 1, res.ErrorCount)
	assert.Equal(t, 0, res.SuccessCount)
}
