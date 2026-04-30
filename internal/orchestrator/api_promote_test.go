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

func TestAPIPromoteBulkJobs(t *testing.T) {
	poller := newMockPoller([]WorkItem{})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	orch.pendingJobs["job1"] = JobInfo{
		WorkItem: WorkItem{ID: "job1", Priority: 5, Tags: []string{"tag1"}},
	}
	orch.pendingJobs["job2"] = JobInfo{
		WorkItem: WorkItem{ID: "job2", Priority: 10, Tags: []string{"tag1"}},
	}
	orch.pendingJobs["job3"] = JobInfo{
		WorkItem: WorkItem{ID: "job3", Priority: 2, Tags: []string{"tag2"}, Description: "match-this-desc"},
	}
	orch.pendingJobs["match-job"] = JobInfo{
		WorkItem: WorkItem{ID: "match-job", Priority: 1},
	}
	orch.pendingJobs["group-job1"] = JobInfo{
		WorkItem: WorkItem{ID: "group-job1", Priority: 15, ConcurrencyGroup: "group1"},
	}
	orch.pendingJobs["group-job2"] = JobInfo{
		WorkItem: WorkItem{ID: "group-job2", Priority: 20, ConcurrencyGroup: "group1"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Promote valid jobs by tag
	resp, err := http.Post(server.URL+"/jobs/promote/bulk?tag=tag1", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), result["promoted"])
	resp.Body.Close()

	// Validate internal state (max priority was 20, should become 21)
	assert.Equal(t, 21, orch.pendingJobs["job1"].WorkItem.Priority)
	assert.Equal(t, 21, orch.pendingJobs["job2"].WorkItem.Priority)
	assert.Equal(t, 2, orch.pendingJobs["job3"].WorkItem.Priority) // unchanged

	// Promote valid jobs by match
	resp, err = http.Post(server.URL+"/jobs/promote/bulk?match=match-.*", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var resultMatch map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resultMatch)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), resultMatch["promoted"])
	resp.Body.Close()

	// Validate internal state (max priority was 21, should become 22)
	assert.Equal(t, 22, orch.pendingJobs["job3"].WorkItem.Priority)
	assert.Equal(t, 22, orch.pendingJobs["match-job"].WorkItem.Priority)

	// Promote valid jobs by group
	resp, err = http.Post(server.URL+"/jobs/promote/bulk?group=group1", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var resultGroup map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resultGroup)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), resultGroup["promoted"])
	resp.Body.Close()

	// Validate internal state (max priority was 22, should become 23)
	assert.Equal(t, 23, orch.pendingJobs["group-job1"].WorkItem.Priority)
	assert.Equal(t, 23, orch.pendingJobs["group-job2"].WorkItem.Priority)

	// Promote missing query param
	respMissing, err := http.Post(server.URL+"/jobs/promote/bulk", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, respMissing.StatusCode)
	respMissing.Body.Close()
}
