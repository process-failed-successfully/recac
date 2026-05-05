package main

import (
	"context"
	"os"
	"testing"
	"bytes"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"net/http"
	"net/http/httptest"
)

func TestMainRun_WaitIdle_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"active_jobs": 0, "pending_jobs": 0}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	viper.Reset()
	viper.Set("orchestrator.wait_idle", true)
	viper.Set("orchestrator.host", server.URL)

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := run(ctx, logger)
	assert.NoError(t, err)
}
