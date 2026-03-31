package orchestrator

import (
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

func TestAPISummary(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Add some jobs with different statuses
	orch.activeJobs["job-1"] = JobInfo{ID: "job-1", Status: "Running"}
	orch.activeJobs["job-2"] = JobInfo{ID: "job-2", Status: "Running"}
	orch.pendingJobs["job-3"] = JobInfo{ID: "job-3", Status: "Pending"}
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "job-4", Status: "Completed"})
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "job-5", Status: "Failed"})
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "job-6", Status: "Failed"})

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	req := httptest.NewRequest(http.MethodGet, "/jobs/summary", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var summary map[string]int
	err := json.Unmarshal(rr.Body.Bytes(), &summary)
	require.NoError(t, err)

	assert.Equal(t, 2, summary["Running"])
	assert.Equal(t, 1, summary["Pending"])
	assert.Equal(t, 1, summary["Completed"])
	assert.Equal(t, 2, summary["Failed"])
	assert.Len(t, summary, 4)
}
