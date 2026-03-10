package orchestrator

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_SetJobOutput(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch.activeJobs["job-1"] = JobInfo{ID: "job-1"}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	reqBody := `{"outputs": {"key1": "val1"}}`
	resp, err := http.Post(server.URL+"/jobs/job-1/output", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("Failed to execute post request: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	job, err := orch.GetJob("job-1")
	assert.NoError(t, err)
	assert.Equal(t, "val1", job.Outputs["key1"])
}

func TestAPI_SetJobOutput_NotFound(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	reqBody := `{"outputs": {"key1": "val1"}}`
	resp, err := http.Post(server.URL+"/jobs/missing/output", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("Failed to execute post request: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
