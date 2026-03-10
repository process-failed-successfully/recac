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
	"github.com/stretchr/testify/require"
)

func TestAPI_SetJobProgress(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Create an active job
	orch.activeJobs["JOB-PROGRESS"] = JobInfo{
		ID:        "JOB-PROGRESS",
		Summary:   "Test Job",
		StartTime: time.Now(),
		Status:    "Running",
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, ctx)

	t.Run("Valid Progress Update", func(t *testing.T) {
		progress := 50
		statusMsg := "Halfway done"

		reqBody := struct {
			Progress      *int    `json:"progress"`
			StatusMessage *string `json:"status_message"`
		}{
			Progress:      &progress,
			StatusMessage: &statusMsg,
		}

		payload, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPut, "/jobs/JOB-PROGRESS/progress", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify state update
		job, err := orch.GetJob("JOB-PROGRESS")
		require.NoError(t, err)
		require.NotNil(t, job.Progress)
		assert.Equal(t, 50, *job.Progress)
		require.NotNil(t, job.StatusMessage)
		assert.Equal(t, "Halfway done", *job.StatusMessage)
	})

	t.Run("Job Not Found", func(t *testing.T) {
		progress := 50
		reqBody := struct {
			Progress *int `json:"progress"`
		}{
			Progress: &progress,
		}

		payload, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest(http.MethodPut, "/jobs/JOB-UNKNOWN/progress", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPut, "/jobs/JOB-PROGRESS/progress", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
