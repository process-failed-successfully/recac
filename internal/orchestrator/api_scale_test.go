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
)

func TestScaleConcurrencyAPI(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	orch.MaxConcurrentJobs = 2

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test scaling up
	reqBody := `{"max_concurrent_jobs": 5}`
	resp, err := http.Post(server.URL+"/scale", "application/json", bytes.NewBufferString(reqBody))
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respData map[string]int
	err = json.NewDecoder(resp.Body).Decode(&respData)
	assert.NoError(t, err)
	assert.Equal(t, 5, respData["max_concurrent_jobs"])

	assert.Equal(t, 5, orch.MaxConcurrentJobs)

	// Test scaling down
	reqBody = `{"max_concurrent_jobs": 1}`
	resp, err = http.Post(server.URL+"/scale", "application/json", bytes.NewBufferString(reqBody))
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, orch.MaxConcurrentJobs)

	// Test invalid body
	reqBody = `{"max_concurrent_jobs": "invalid"}`
	resp, err = http.Post(server.URL+"/scale", "application/json", bytes.NewBufferString(reqBody))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Test negative concurrency
	reqBody = `{"max_concurrent_jobs": -1}`
	resp, err = http.Post(server.URL+"/scale", "application/json", bytes.NewBufferString(reqBody))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestScaleConcurrencyEvaluatesPendingJobs(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	orch.MaxConcurrentJobs = 1

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add an active job to hit capacity
	orch.activeJobs["job-1"] = JobInfo{
		ID:        "job-1",
		Status:    "Active",
		StartTime: time.Now(),
		WorkItem: WorkItem{
			ID: "job-1",
		},
	}
	orch.activeSpawns = 1

	// Add a pending job
	orch.pendingJobs["job-2"] = JobInfo{
		ID:        "job-2",
		Status:    "Pending",
		StartTime: time.Now(),
		WorkItem: WorkItem{
			ID: "job-2",
		},
	}

	// Make sure mock handles spawn
	mockSpawner.On("Spawn", context.Background(), WorkItem{ID: "job-2"}).Return(nil)

	// Scale up
	orch.SetConcurrency(context.Background(), 2, logger)

	// Wait for the job to be processed (moved out of pending)
	assert.Eventually(t, func() bool {
		orch.mu.RLock()
		defer orch.mu.RUnlock()
		_, isPending := orch.pendingJobs["job-2"]
		return !isPending
	}, 2*time.Second, 10*time.Millisecond, "Job should be moved out of pending queue")

	// Verify the job is either active or completed
	job, err := orch.GetJob("job-2")
	assert.NoError(t, err)

	orch.mu.RLock()
	defer orch.mu.RUnlock()

	_, active := orch.activeJobs["job-2"]
	if !active {
		found := false
		for _, completed := range orch.completedJobs {
			if completed.ID == "job-2" {
				found = true
				break
			}
		}
		assert.True(t, found, "Job should be in completed jobs history if not active")
	} else {
		assert.Equal(t, "Spawning", job.Status)
	}
}
