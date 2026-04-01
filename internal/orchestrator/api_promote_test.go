package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
    "log/slog"
    "os"

	"github.com/stretchr/testify/assert"
)

func TestAPIPromoteJob(t *testing.T) {
	poller := newMockPoller([]WorkItem{})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	orch.pendingJobs["job1"] = JobInfo{
		WorkItem: WorkItem{ID: "job1", Priority: 5},
	}
	orch.pendingJobs["job2"] = JobInfo{
		WorkItem: WorkItem{ID: "job2", Priority: 10},
	}

    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    mux := http.NewServeMux()
    RegisterAPI(mux, orch, logger, context.Background())
    server := httptest.NewServer(mux)
	defer server.Close()

	// Promote valid job
	resp, err := http.Post(server.URL+"/jobs/job1/promote", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "job1", result["id"])
	assert.Equal(t, float64(11), result["priority"])
	resp.Body.Close()

	// Validate internal state
	assert.Equal(t, 11, orch.pendingJobs["job1"].WorkItem.Priority)

	// Promote missing job
	respMissing, err := http.Post(server.URL+"/jobs/missing/promote", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, respMissing.StatusCode)
	respMissing.Body.Close()
}
