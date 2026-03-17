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

func TestAPI_UpdateJobsTimeoutBulk(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	setupOrch := func() (*Orchestrator, *http.ServeMux) {
		orch := New(nil, nil, time.Minute)
		orch.pendingJobs = map[string]JobInfo{
			"job1": {
				ID:      "job1",
				Summary: "Fix bug 1",
				WorkItem: WorkItem{
					ID:       "job1",
					Tags:     []string{"backend", "urgent"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
			"job2": {
				ID:      "job2",
				Summary: "Update frontend",
				WorkItem: WorkItem{
					ID:       "job2",
					Tags:     []string{"frontend"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
			"job3": {
				ID:      "job3",
				Summary: "Fix bug 2",
				WorkItem: WorkItem{
					ID:       "job3",
					Tags:     []string{"backend"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
		}

		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, ctx)
		return orch, mux
	}

	t.Run("ByTag", func(t *testing.T) {
		orch, mux := setupOrch()

		body := `{"timeout": "30m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?tag=backend", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"updated": 2`)

		orch.mu.RLock()
		defer orch.mu.RUnlock()

		assert.Equal(t, 30*time.Minute, orch.pendingJobs["job1"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job2"].WorkItem.Timeout)
		assert.Equal(t, 30*time.Minute, orch.pendingJobs["job3"].WorkItem.Timeout)
	})

	t.Run("ByMatch", func(t *testing.T) {
		orch, mux := setupOrch()

		body := `{"timeout": "45m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?match=Fix", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"updated": 2`)

		orch.mu.RLock()
		defer orch.mu.RUnlock()

		assert.Equal(t, 45*time.Minute, orch.pendingJobs["job1"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job2"].WorkItem.Timeout)
		assert.Equal(t, 45*time.Minute, orch.pendingJobs["job3"].WorkItem.Timeout)
	})

	t.Run("MissingQueryParams", func(t *testing.T) {
		_, mux := setupOrch()

		body := `{"timeout": "45m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Either 'tag' or 'match' query parameter is required")
	})

	t.Run("BothQueryParams", func(t *testing.T) {
		_, mux := setupOrch()

		body := `{"timeout": "45m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?tag=backend&match=Fix", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Cannot provide both")
	})

	t.Run("InvalidRegex", func(t *testing.T) {
		_, mux := setupOrch()

		body := `{"timeout": "45m"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?match=[invalid", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid match regex")
	})

	t.Run("InvalidTimeoutFormat", func(t *testing.T) {
		_, mux := setupOrch()

		body := `{"timeout": "invalid"}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?tag=backend", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid timeout format")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		_, mux := setupOrch()

		body := `{"timeout":`
		req := httptest.NewRequest(http.MethodPut, "/jobs/timeout?tag=backend", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid JSON body")
	})
}
