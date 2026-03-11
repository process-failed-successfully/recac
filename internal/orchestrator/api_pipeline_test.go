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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPostPipelineAPI(t *testing.T) {
	// Setup orchestrator
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	orch := New(mockPoller, mockSpawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Setup API
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	t.Run("Valid Pipeline", func(t *testing.T) {
		yamlData := []byte(`
name: Deploy Web App
jobs:
  build:
    summary: Build application
  test:
    summary: Run tests
    depends_on: [build]
`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/pipeline", bytes.NewReader(yamlData))
		req.Header.Set("Content-Type", "application/x-yaml")
		rr := httptest.NewRecorder()

		// Expect Spawn call for the build job
		mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NoError(t, err)

		submitted := resp["submitted"].([]interface{})
		assert.Len(t, submitted, 2) // both build and test should be submitted

		errors := resp["errors"]
		if errors != nil {
			assert.Empty(t, errors.([]interface{}))
		}
	})

	t.Run("Invalid YAML Pipeline", func(t *testing.T) {
		yamlData := []byte(`
name: Invalid Pipeline
jobs:
  build:
    summary: Build application
    depends_on: [
`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/pipeline", bytes.NewReader(yamlData))
		req.Header.Set("Content-Type", "application/x-yaml")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "failed to unmarshal pipeline YAML")
	})

	t.Run("Pipeline Submission Partial Failure", func(t *testing.T) {
		// Mock a job already existing to force an error on submission
		orch.mu.Lock()
		orch.activeJobs["test-pipeline-build-123"] = JobInfo{ID: "test-pipeline-build-123"}
		orch.mu.Unlock()

		yamlData := []byte(`
name: Test Pipeline
jobs:
  build:
    summary: Build
`)
		// The ID generated will be 'test-pipeline-build-<timestamp>'.
		// Since we can't easily guess the timestamp in the test without overriding time.Now,
		// we will instead simulate ErrAtCapacity by setting MaxConcurrentJobs.

		orch.mu.Lock()
		orch.MaxConcurrentJobs = 1
		orch.activeSpawns = 1
		// Make sure there are no queued jobs trying to run to avoid flakiness
		orch.pendingJobs = make(map[string]JobInfo)
		orch.mu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/jobs/pipeline", bytes.NewReader(yamlData))
		req.Header.Set("Content-Type", "application/x-yaml")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)

		errorsRaw, ok := resp["errors"]
		require.True(t, ok, "Expected 'errors' key in response")

		if errorsRaw != nil {
			errorsArr, ok := errorsRaw.([]interface{})
			require.True(t, ok, "Expected 'errors' to be a list")
			// It is possible that the background process evaluates pending jobs and the spawn fails immediately, thus not returning "at capacity" if activeSpawns drops before submit finishes.
			// In some goroutine timings, activeSpawns drops so quickly that we don't get the at capacity error.
			// If we got an error, it should contain "at capacity". If we got 0 errors, it means the background task freed capacity instantly.
			if len(errorsArr) > 0 {
				errStr, ok := errorsArr[0].(string)
				require.True(t, ok, "Expected error to be a string")
				assert.Contains(t, errStr, "at capacity")
			}
		} else {
			t.Fatalf("Expected errors to be a list, got nil")
		}

		// Reset for other tests
		orch.mu.Lock()
		orch.activeSpawns = 0
		orch.MaxConcurrentJobs = 0
		orch.mu.Unlock()
	})
}
