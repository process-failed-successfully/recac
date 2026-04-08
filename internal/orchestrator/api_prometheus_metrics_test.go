package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrometheusMetricsAPI(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Simulate some state
	orch.mu.Lock()
	orch.activeJobs["JOB-1"] = JobInfo{ID: "JOB-1", Status: "Running"}
	orch.activeJobs["JOB-2"] = JobInfo{ID: "JOB-2", Status: "Running"}
	orch.pendingJobs["JOB-3"] = JobInfo{ID: "JOB-3", Status: "Pending"}
	orch.totalSpawns = 42
	orch.activeSpawns = 2
	orch.paused = true
	orch.mu.Unlock()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/plain; version=0.0.4", rr.Header().Get("Content-Type"))

	body := rr.Body.String()

	assert.Contains(t, body, "recac_active_jobs 2")
	assert.Contains(t, body, "recac_pending_jobs 1")
	assert.Contains(t, body, "recac_total_spawns 42")
	assert.Contains(t, body, "recac_active_spawns 2")
	assert.Contains(t, body, "recac_paused 1")
	assert.Contains(t, body, "recac_draining 0")

	// Test unpausing and draining
	orch.mu.Lock()
	orch.paused = false
	orch.draining = true
	orch.mu.Unlock()

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req)

	body2 := rr2.Body.String()
	assert.Contains(t, body2, "recac_paused 0")
	assert.Contains(t, body2, "recac_draining 1")
}
