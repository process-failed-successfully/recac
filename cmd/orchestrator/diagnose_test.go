package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestDiagnoseCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/diagnose", func(w http.ResponseWriter, r *http.Request) {
		report := orchestrator.DiagnosticReport{
			UnresolvableJobs: []orchestrator.UnresolvableJob{
				{
					JobID:       "job-dead",
					MissingDeps: []string{"ghost"},
				},
			},
		}
		json.NewEncoder(w).Encode(report)
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
	viper.Set("orchestrator.diagnose", true)
	viper.Set("orchestrator.host", server.URL)
	// Disable other flags
	viper.Set("orchestrator.scale", -1)
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

	output := out.String()
	assert.Contains(t, output, "Diagnostic Report")
	assert.Contains(t, output, "Unresolvable Jobs")
	assert.Contains(t, output, "job-dead")
	assert.Contains(t, output, "ghost")

	// Healthy system
	t.Run("HealthySystem", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(orchestrator.DiagnosticReport{})
		}))
		defer ts.Close()

		exitCode = 0
		out.Reset()
		runDiagnose(ts.URL)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, out.String(), "No issues found! The system is healthy.")
	})

	// Deadlocked system
	t.Run("DeadlockedSystem", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			report := orchestrator.DiagnosticReport{
				DeadlockedJobs: []orchestrator.DeadlockedJob{
					{
						JobID: "job-1",
						Cycle: []string{"job-1", "job-2", "job-1"},
					},
				},
			}
			json.NewEncoder(w).Encode(report)
		}))
		defer ts.Close()

		exitCode = 0
		out.Reset()
		runDiagnose(ts.URL)
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, out.String(), "Deadlocks / Cyclic Dependencies (1)")
		assert.Contains(t, out.String(), "job-1")
		assert.Contains(t, out.String(), "job-1 -> job-2 -> job-1")
	})

	// Connection failure
	t.Run("ConnectionFailure", func(t *testing.T) {
		exitCode = 0
		out.Reset()
		runDiagnose("http://127.0.0.1:0")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to connect to orchestrator")
	})

	// Non-200 OK
	t.Run("Non200OK", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer ts.Close()

		exitCode = 0
		out.Reset()
		runDiagnose(ts.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to fetch diagnostics: internal error")
	})

	// Invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer ts.Close()

		exitCode = 0
		out.Reset()
		runDiagnose(ts.URL)
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, out.String(), "Failed to decode response")
	})
}
