package orchestrator

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateTimeoutAPI(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	orch.pendingJobs["job1"] = JobInfo{
		ID: "job1",
		WorkItem: WorkItem{
			ID:       "job1",
			RunAfter: time.Now().Add(1 * time.Hour), // Keep it pending
		},
	}
	orch.activeJobs["job2"] = JobInfo{
		ID: "job2",
		WorkItem: WorkItem{
			ID: "job2",
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	t.Run("Success", func(t *testing.T) {
		body := `{"timeout": "20m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/job1/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"timeout": "20m0s"`)

		orch.mu.RLock()
		job := orch.pendingJobs["job1"]
		orch.mu.RUnlock()
		assert.Equal(t, 20*time.Minute, job.WorkItem.Timeout)
	})

	t.Run("InvalidDuration", func(t *testing.T) {
		body := `{"timeout": "invalid"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/job1/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid timeout format")
	})

	t.Run("JobNotFound", func(t *testing.T) {
		body := `{"timeout": "20m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/missing-job/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("JobAlreadyActive", func(t *testing.T) {
		body := `{"timeout": "20m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/job2/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		body := `{"timeout": `
		req := httptest.NewRequest(http.MethodPut, "/jobs/job1/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid JSON body")
	})
}
