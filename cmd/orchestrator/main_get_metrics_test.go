package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestMainGetJobMetrics(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-789", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		job := orchestrator.JobInfo{
			ID: "JOB-789",
			Metrics: map[string]float64{
				"total_cost": 42.5,
			},
		}
		json.NewEncoder(w).Encode(job)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var outBuf bytes.Buffer
	originalStdout := stdout
	stdout = &outBuf
	defer func() { stdout = originalStdout }()

	// Reset viper
	viper.Reset()

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	}()

	// Simulate flags
	os.Args = []string{"orchestrator", "--get-metrics-job", "JOB-789", "--get-metrics-key", "total_cost", "--host", server.URL}
	pflag.CommandLine = pflag.NewFlagSet("orchestrator", pflag.ContinueOnError)

	// Temporarily redirect main's output to prevent polluting test logs
	oldOut := os.Stdout
	os.Stdout = os.NewFile(0, os.DevNull)
	defer func() { os.Stdout = oldOut }()

	main()

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, outBuf.String(), "42.5\n")
}

func TestMainGetJobMetrics_MissingKeyFlag(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	var outBuf bytes.Buffer
	originalStdout := stdout
	stdout = &outBuf
	defer func() { stdout = originalStdout }()

	// Reset viper
	viper.Reset()

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	}()

	// Simulate flags missing key
	os.Args = []string{"orchestrator", "--get-metrics-job", "JOB-789", "--host", "http://localhost:2112"}
	pflag.CommandLine = pflag.NewFlagSet("orchestrator", pflag.ContinueOnError)

	// Temporarily redirect main's output to prevent polluting test logs
	oldOut := os.Stdout
	os.Stdout = os.NewFile(0, os.DevNull)
	defer func() { os.Stdout = oldOut }()

	main()

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, outBuf.String(), "Error: --get-metrics-key is required when using --get-metrics-job")
}

func TestMainGetJobMetrics_MissingKeyData(t *testing.T) {
	var exitCode int
	originalExit := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-789", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		job := orchestrator.JobInfo{
			ID: "JOB-789",
			Metrics: map[string]float64{
				"total_cost": 42.5,
			},
		}
		json.NewEncoder(w).Encode(job)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var outBuf bytes.Buffer
	originalStdout := stdout
	stdout = &outBuf
	defer func() { stdout = originalStdout }()

	// Reset viper
	viper.Reset()

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	}()

	// Simulate flags
	os.Args = []string{"orchestrator", "--get-metrics-job", "JOB-789", "--get-metrics-key", "missing_metric", "--host", server.URL}
	pflag.CommandLine = pflag.NewFlagSet("orchestrator", pflag.ContinueOnError)

	// Temporarily redirect main's output to prevent polluting test logs
	oldOut := os.Stdout
	os.Stdout = os.NewFile(0, os.DevNull)
	defer func() { os.Stdout = oldOut }()

	main()

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, outBuf.String(), "Metrics key 'missing_metric' not found for job JOB-789")
}
