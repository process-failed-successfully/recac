package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Spawner (MockPoller is defined in spawner_docker_test.go)
type MockSpawner struct {
	mock.Mock
}

func (m *MockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSpawner) Cancel(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	args := m.Called(ctx, jobID)
	// Return a dummy ReadCloser
	if r, ok := args.Get(0).(io.ReadCloser); ok {
		return r, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSpawner) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestRegisterAPI(t *testing.T) {
	// Setup
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()

	// Register API with mux
	RegisterAPI(mux, orch, logger, context.Background())
	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Test /status
	t.Run("Status Endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/status")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var status Status
		err = json.NewDecoder(resp.Body).Decode(&status)
		assert.NoError(t, err)
		assert.Equal(t, "1m0s", status.PollInterval)
	})

	// 2. Test /jobs (empty)
	t.Run("Jobs Endpoint Empty", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var jobs []JobInfo
		err = json.NewDecoder(resp.Body).Decode(&jobs)
		assert.NoError(t, err)
		assert.Empty(t, jobs)
	})

	// 3. Test Submit Job
	item := WorkItem{ID: "JOB-123", Summary: "Test Job"}
	t.Run("Submit Job", func(t *testing.T) {
		body, _ := json.Marshal(item)

		// Expect Spawn call
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-123"
		})).Return(nil)

		// Expect Cleanup call eventually (but we might not wait for it here)
		mockSpawner.On("Cleanup", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-123"
		})).Return(nil).Maybe()

		resp, err := http.Post(server.URL + "/jobs", "application/json", strings.NewReader(string(body)))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		// Verify Job is Active or Completed
		time.Sleep(10 * time.Millisecond)

		resp, err = http.Get(server.URL + "/jobs?state=all")
		var jobs []JobInfo
		json.NewDecoder(resp.Body).Decode(&jobs)
		assert.NotEmpty(t, jobs)
		assert.Equal(t, "JOB-123", jobs[0].ID)
	})

	// 4. Test Get Job Details
	t.Run("Get Job Details", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/jobs/JOB-123")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var job JobInfo
		err = json.NewDecoder(resp.Body).Decode(&job)
		assert.NoError(t, err)
		assert.Equal(t, "JOB-123", job.ID)
	})

	// 5. Test Get Logs
	t.Run("Get Logs", func(t *testing.T) {
		// Mock GetLogs
		mockLogs := io.NopCloser(strings.NewReader("Log content"))
		mockSpawner.On("GetLogs", mock.Anything, "JOB-123").Return(mockLogs, nil)

		resp, err := http.Get(server.URL + "/jobs/JOB-123/logs")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "Log content", string(body))
	})

	// 6. Test Cancel Job
	t.Run("Cancel Job", func(t *testing.T) {
		// Expect Cancel call
		mockSpawner.On("Cancel", mock.Anything, "JOB-123").Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, server.URL + "/jobs/JOB-123", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 6.5 Test Cancel All Jobs
	t.Run("Cancel All Jobs", func(t *testing.T) {
		// Mock Cancel to return nil for any job
		mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)

		// Insert a dummy active job to be canceled
		orch.mu.Lock()
		orch.activeJobs["JOB-999"] = JobInfo{
			ID: "JOB-999",
			Status: "Running",
		}
		orch.mu.Unlock()

		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/jobs", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]int
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, result["canceled"], 1)
	})

	// 7. Test Pause/Resume
	t.Run("Pause Resume", func(t *testing.T) {
		resp, err := http.Post(server.URL + "/pause", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		orch.mu.Lock()
		paused := orch.paused
		orch.mu.Unlock()
		assert.True(t, paused)

		resp, err = http.Post(server.URL + "/resume", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		orch.mu.Lock()
		paused = orch.paused
		orch.mu.Unlock()
		assert.False(t, paused)
	})

	// 8. Test Retry
	t.Run("Retry Failed", func(t *testing.T) {
		// Manually insert a failed job into history
		orch.mu.Lock()
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "JOB-FAIL",
			Status: "Failed",
			WorkItem: WorkItem{ID: "JOB-FAIL"},
		})
		orch.mu.Unlock()

		// Expect Spawn call for retry
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-FAIL"
		})).Return(nil)

		resp, err := http.Post(server.URL + "/jobs/JOB-FAIL/retry", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	// 9. Retry All Failed
	t.Run("Retry All Failed", func(t *testing.T) {
		// Manually insert another failed job
		orch.mu.Lock()
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "JOB-FAIL-2",
			Status: "Failed",
			WorkItem: WorkItem{ID: "JOB-FAIL-2"},
		})
		orch.mu.Unlock()

		// Expect Spawn
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
			return i.ID == "JOB-FAIL-2"
		})).Return(nil)

		resp, err := http.Post(server.URL + "/jobs/retry-failed", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Check response body
		var result map[string]int
		json.NewDecoder(resp.Body).Decode(&result)
		// Should be at least 1 (JOB-FAIL-2)
		assert.GreaterOrEqual(t, result["retried"], 1)
	})
}
