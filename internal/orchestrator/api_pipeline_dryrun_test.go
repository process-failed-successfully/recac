package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_PipelineDryRun(t *testing.T) {
	orch := New(&mockPoller{}, &MockSpawner{}, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	yamlData := []byte(`
name: test-pipeline
jobs:
  job1:
    summary: "Job 1"
    repo_url: "https://github.com/test/repo"
`)

	req := httptest.NewRequest(http.MethodPost, "/jobs/pipeline/dry-run", bytes.NewReader(yamlData))
	req.Header.Set("Content-Type", "application/x-yaml")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var items []WorkItem
	err := json.Unmarshal(rr.Body.Bytes(), &items)
	require.NoError(t, err)

	assert.Len(t, items, 1)
	assert.Equal(t, "Job 1", items[0].Summary)
	assert.Contains(t, items[0].ID, "test-pipeline-job1")

	// Ensure no jobs were actually submitted to the orchestrator
	assert.Len(t, orch.GetActiveJobs(), 0)
	assert.Len(t, orch.GetPendingJobs(), 0)
}
