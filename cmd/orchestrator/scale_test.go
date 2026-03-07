package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestScaleCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/scale", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		}

		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, 5, req.MaxConcurrentJobs)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"max_concurrent_jobs": %d}`, req.MaxConcurrentJobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	viper.Reset()
	viper.Set("orchestrator.scale", 5)
	viper.Set("orchestrator.host", server.URL)

	// Disable other flags that might interfere
	viper.Set("orchestrator.list_jobs", false)
	viper.Set("orchestrator.status", false)
	viper.Set("orchestrator.cancel_all", false)
	defer viper.Reset()

	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(context.Background(), logger)

	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Orchestrator concurrency limit scaled to 5.")
}

func TestScaleConcurrency_Errors(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	originalStdout := stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = originalStdout }()

	t.Run("ConnectionError", func(t *testing.T) {
		exitCode = 0
		buf.Reset()
		scaleConcurrency("http://invalid-host", 5)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("BadStatusCode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		exitCode = 0
		buf.Reset()
		scaleConcurrency(server.URL, 5)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to scale orchestrator")
	})

}
