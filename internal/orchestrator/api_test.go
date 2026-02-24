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
	return args.Get(0).(io.ReadCloser), args.Error(1)
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
	t.Run("Submit Job", func(t *testing.T) {
		item := WorkItem{ID: "JOB-123", Summary: "Test Job"}
		body, _ := json.Marshal(item)

		// Expect Spawn call
		mockSpawner.On("Spawn", mock.Anything, item).Return(nil)

		resp, err := http.Post(server.URL+"/jobs", "application/json", strings.NewReader(string(body)))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		// Verify Job is Active or Completed (since mock returns immediately, it might be completed)
		resp, err = http.Get(server.URL + "/jobs?state=all")
		var jobs []JobInfo
		json.NewDecoder(resp.Body).Decode(&jobs)
		assert.NotEmpty(t, jobs)
		assert.Equal(t, "JOB-123", jobs[0].ID)
	})

	// 4. Test Cancel Job
	t.Run("Cancel Job", func(t *testing.T) {
		// Expect Cancel call
		mockSpawner.On("Cancel", mock.Anything, "JOB-123").Return(nil)

		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/jobs/JOB-123", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
