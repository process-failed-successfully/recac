package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestPauseResumeGroup(t *testing.T) {
	logger := slog.Default()
	o := orchestrator.New(nil, nil, time.Minute)

	mux := http.NewServeMux()
	orchestrator.RegisterAPI(mux, o, logger, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	// Pause group
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/groups/test-group/pause", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Resume group
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/groups/test-group/resume", nil)
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
