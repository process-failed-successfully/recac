package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAPI_SkipJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "S1"}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Call API to Skip Job
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/J1/skip", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "J1", completed[0].ID)
	assert.Equal(t, "Skipped", completed[0].Status)

	// Call API to Skip Non-existent Job
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/jobs/NON-EXISTENT/skip", nil)
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAPI_BulkSkip(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "S1", Tags: []string{"tag1"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J2", Summary: "S2", Tags: []string{"tag1", "tag2"}}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, ctx)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip by Tag
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?tag=tag1", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "Skipped", completed[0].Status)
	assert.Equal(t, "Skipped", completed[1].Status)
}

func TestAPI_BulkSkip_InvalidRegex(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip by Invalid Match
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?match=[invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPI_BulkSkip_MissingParam(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip without Tag, Match or Group
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPI_BulkSkipByGroup(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true

	orch.SubmitJob(context.Background(), WorkItem{ID: "J1", ConcurrencyGroup: "group1"}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "J2", ConcurrencyGroup: "group2"}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "J3", ConcurrencyGroup: "group1"}, nil)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// Call Bulk Skip by Group
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?group=group1", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "Skipped", completed[0].Status)
	assert.Equal(t, "Skipped", completed[1].Status)

	pending := orch.GetPendingJobs()
	assert.Len(t, pending, 1)
	assert.Equal(t, "J2", pending[0].ID)
}

func TestSkipJobDownstreamAPI(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	orch.pendingJobs["A"] = JobInfo{
		ID:       "A",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "A"},
	}
	orch.pendingJobs["B"] = JobInfo{
		ID:       "B",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "B", DependsOn: []string{"A"}},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/A/skip?downstream=true", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify they are skipped
	orch.mu.RLock()
	_, okA := orch.pendingJobs["A"]
	_, okB := orch.pendingJobs["B"]
	orch.mu.RUnlock()
	assert.False(t, okA)
	assert.False(t, okB)
}

func TestSkipJobDownstreamAPI_NotFound(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/UNKNOWN/skip?downstream=true", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSkipJobsOlderThanAPI(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	// Add an old job
	job1 := JobInfo{
		ID:        "JOB1",
		Status:    "Pending",
		StartTime: time.Now().Add(-2 * time.Hour),
	}
	orch.pendingJobs["JOB1"] = job1

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?older_than=1h", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]int
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, 1, result["skipped"])

	// Verify job was skipped
	orch.mu.Lock()
	_, exists := orch.pendingJobs["JOB1"]
	orch.mu.Unlock()
	assert.False(t, exists)
}

func TestSkipJobsOlderThanAPI_InvalidDuration(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/jobs/skip?older_than=invalid", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
