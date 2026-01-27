package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestLoadCommand(t *testing.T) {
	// 1. Start Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate latency
		if r.URL.Path == "/slow" {
			time.Sleep(50 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Reset globals after test
	defer func() {
		loadRate = 0
		loadDuration = 0
		loadMethod = ""
		loadThreshold = ""
	}()

	t.Run("Basic Load Test", func(t *testing.T) {
		loadRate = 10
		loadDuration = 500 * time.Millisecond
		loadMethod = "GET"
		loadThreshold = ""

		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)

		err := runLoad(cmd, []string{ts.URL})
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "Starting load test")
		assert.Contains(t, out.String(), "Success Rate")
	})

	t.Run("Threshold Pass", func(t *testing.T) {
		loadRate = 10
		loadDuration = 500 * time.Millisecond
		loadMethod = "GET"
		loadThreshold = "p95<100ms"

		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)

		err := runLoad(cmd, []string{ts.URL})
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "Threshold check passed")
	})

	t.Run("Threshold Fail", func(t *testing.T) {
		loadRate = 10
		loadDuration = 500 * time.Millisecond
		loadMethod = "GET"
		loadThreshold = "p95<1ms" // 50ms latency > 1ms

		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)

		err := runLoad(cmd, []string{ts.URL + "/slow"})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "threshold failed")
		}
	})
}

func TestProcessResults(t *testing.T) {
	results := make(chan LoadResult, 10)
	results <- LoadResult{Latency: 10 * time.Millisecond, Status: 200}
	results <- LoadResult{Latency: 20 * time.Millisecond, Status: 200}
	results <- LoadResult{Latency: 30 * time.Millisecond, Status: 500}
	results <- LoadResult{Error: assert.AnError}
	close(results)

	stats := processResultsStreaming(results)

	assert.Equal(t, 4, stats.TotalRequests)
	assert.Equal(t, 3, stats.Success) // 200, 200, 500 (HTTP success in terms of no net error)
	assert.Equal(t, 1, stats.Errors)  // Net error
	assert.Equal(t, 2, stats.StatusCodes[200])
	assert.Equal(t, 1, stats.StatusCodes[500])
	assert.Equal(t, 3, len(stats.Latencies)) // Only non-error latencies
}

func TestCheckThresholdLogic(t *testing.T) {
	stats := LoadStats{
		TotalRequests: 100,
		Errors:        5,
		Latencies: []time.Duration{
			10 * time.Millisecond,
			50 * time.Millisecond, // Mean approx 30
			100 * time.Millisecond,
		},
		RPS: 50.0,
	}
	// Latencies sorted: 10, 50, 100.
	// p50 (index 1) = 50
	// p95 (index 2) = 100

	tests := []struct {
		threshold string
		wantErr   bool
	}{
		{"p50 < 60ms", false},
		{"p50 < 40ms", true},
		{"p95 < 200ms", false},
		{"p95 < 80ms", true},
		{"error < 10%", false}, // 5% < 10%
		{"error < 1%", true},
		{"rps > 40", false},
		{"rps > 60", true},
	}

	for _, tt := range tests {
		t.Run(tt.threshold, func(t *testing.T) {
			err := checkThreshold(stats, tt.threshold)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
