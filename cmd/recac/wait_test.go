package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitCmd_Success(t *testing.T) {
	// Create a mock server that returns 200 immediately
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cmd := &cobra.Command{
		Use:  "wait",
		RunE: runWait,
	}

	// We can't rely on flags set in init(), so we set them explicitly here
	waitTimeout = 2 * time.Second
	waitInterval = 100 * time.Millisecond

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{ts.URL})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "is ready!")
}

func TestWaitCmd_DelayedSuccess(t *testing.T) {
	requestCount := 0
	var mu sync.Mutex

	// Create a mock server that returns 503 the first 2 times, then 200
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()

		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cmd := &cobra.Command{
		Use:  "wait",
		RunE: runWait,
	}

	waitTimeout = 2 * time.Second
	waitInterval = 100 * time.Millisecond

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{ts.URL})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "is ready!")

	mu.Lock()
	count := requestCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 3)
}

func TestWaitCmd_Timeout(t *testing.T) {
	// Create a mock server that always returns 500
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := &cobra.Command{
		Use:  "wait",
		RunE: runWait,
	}

	waitTimeout = 500 * time.Millisecond
	waitInterval = 100 * time.Millisecond

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{ts.URL})

	err := cmd.Execute()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "timeout reached"))
}
