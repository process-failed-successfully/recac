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

func TestAPIDemoteJob(t *testing.T) {
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

	// Demote valid job (job2)
	resp, err := http.Post(server.URL+"/jobs/job2/demote", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	// it should return the new priority in the response {"priority": 4} since min was 5
	assert.Equal(t, float64(4), result["priority"])
	resp.Body.Close()

	// Validate internal state
	assert.Equal(t, 4, orch.pendingJobs["job2"].WorkItem.Priority)

	// Demote missing job
	respMissing, err := http.Post(server.URL+"/jobs/missing/demote", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, respMissing.StatusCode)
	respMissing.Body.Close()
}

func TestAPIDemoteBulkJobs(t *testing.T) {
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

	// Demote valid jobs by tag
	resp, err := http.Post(server.URL+"/jobs/demote/bulk?tag=tag1", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), result["demoted"])
	resp.Body.Close()

	// Validate internal state (min priority was 1, should become 0)
	assert.Equal(t, 0, orch.pendingJobs["job1"].WorkItem.Priority)
	assert.Equal(t, 0, orch.pendingJobs["job2"].WorkItem.Priority)
	assert.Equal(t, 2, orch.pendingJobs["job3"].WorkItem.Priority) // unchanged

	// Demote valid jobs by match
	resp, err = http.Post(server.URL+"/jobs/demote/bulk?match=match-.*", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var resultMatch map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resultMatch)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), resultMatch["demoted"])
	resp.Body.Close()

	// Validate internal state (min priority was 0, should become -1)
	assert.Equal(t, -1, orch.pendingJobs["job3"].WorkItem.Priority)
	assert.Equal(t, -1, orch.pendingJobs["match-job"].WorkItem.Priority)

	// Demote valid jobs by group
	resp, err = http.Post(server.URL+"/jobs/demote/bulk?group=group1", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var resultGroup map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&resultGroup)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), resultGroup["demoted"])
	resp.Body.Close()

	// Validate internal state (min priority was -1, should become -2)
	assert.Equal(t, -2, orch.pendingJobs["group-job1"].WorkItem.Priority)
	assert.Equal(t, -2, orch.pendingJobs["group-job2"].WorkItem.Priority)

	// Demote missing query param
	respMissing, err := http.Post(server.URL+"/jobs/demote/bulk", "application/json", nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, respMissing.StatusCode)
	respMissing.Body.Close()
}
