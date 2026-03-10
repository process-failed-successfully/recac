package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostMetricsAPI(t *testing.T) {
	// Setup orchestrator
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Add a job
	err := orch.SubmitJob(ctx, WorkItem{ID: "METRICS-API-1", Summary: "J1"}, logger)
	require.NoError(t, err)

	// Setup API
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	t.Run("Valid JSON", func(t *testing.T) {
		payload := `{"metrics": {"tokens": 150.5, "cost": 0.02}}`
		req := httptest.NewRequest(http.MethodPost, "/jobs/METRICS-API-1/metrics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "success", resp["status"])
		assert.Equal(t, "METRICS-API-1", resp["job_id"])

		// Verify metric was added
		job, err := orch.GetJob("METRICS-API-1")
		require.NoError(t, err)
		assert.Equal(t, 150.5, job.Metrics["tokens"])
		assert.Equal(t, 0.02, job.Metrics["cost"])
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		payload := `{"metrics": "invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/jobs/METRICS-API-1/metrics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid JSON body")
	})

	t.Run("Job Not Found", func(t *testing.T) {
		payload := `{"metrics": {"cost": 1.0}}`
		req := httptest.NewRequest(http.MethodPost, "/jobs/NON-EXISTENT/metrics", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "job NON-EXISTENT not found")
	})
}
