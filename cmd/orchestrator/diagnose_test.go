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
}
