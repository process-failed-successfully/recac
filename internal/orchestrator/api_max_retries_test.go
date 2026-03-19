package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_UpdateJobMaxRetries(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	ctx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), ctx)

	// Add a pending job
	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:       "JOB-1",
		WorkItem: WorkItem{ID: "JOB-1", DependsOn: []string{"DEP"}},
	}
	orch.mu.Unlock()

	t.Run("Valid Update", func(t *testing.T) {
		reqBody := `{"max_retries": 5}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1/max-retries", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"max_retries": 5`)

		orch.mu.Lock()
		job := orch.pendingJobs["JOB-1"]
		orch.mu.Unlock()
		require.NotNil(t, job.WorkItem.MaxRetries)
		assert.Equal(t, 5, *job.WorkItem.MaxRetries)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		reqBody := `invalid`
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1/max-retries", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Job Not Found", func(t *testing.T) {
		reqBody := `{"max_retries": 5}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/UNKNOWN/max-retries", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestAPI_UpdateJobsMaxRetriesBulk(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	ctx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), ctx)

	// Add pending jobs
	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID: "JOB-1",
		Summary: "Fix bug",
		WorkItem: WorkItem{
			ID:      "JOB-1",
			Tags:    []string{"backend"},
			Summary: "Fix bug",
			DependsOn: []string{"DEP"},
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID: "JOB-2",
		Summary: "Fix bug too",
		WorkItem: WorkItem{
			ID:      "JOB-2",
			Tags:    []string{"frontend"},
			Summary: "Fix bug too",
			DependsOn: []string{"DEP"},
		},
	}
	orch.mu.Unlock()

	t.Run("By Tag", func(t *testing.T) {
		reqBody := `{"max_retries": 10}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/max-retries?tag=backend", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"updated": 1`)
		assert.Contains(t, rr.Body.String(), `"max_retries": 10`)

		orch.mu.Lock()
		job := orch.pendingJobs["JOB-1"]
		orch.mu.Unlock()
		require.NotNil(t, job.WorkItem.MaxRetries)
		assert.Equal(t, 10, *job.WorkItem.MaxRetries)
	})

	t.Run("By Match", func(t *testing.T) {
		reqBody := `{"max_retries": 20}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/max-retries?match=Fix", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Updated    int `json:"updated"`
			MaxRetries int `json:"max_retries"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Updated)
		assert.Equal(t, 20, resp.MaxRetries)

		orch.mu.Lock()
		j1 := orch.pendingJobs["JOB-1"]
		j2 := orch.pendingJobs["JOB-2"]
		orch.mu.Unlock()

		require.NotNil(t, j1.WorkItem.MaxRetries)
		assert.Equal(t, 20, *j1.WorkItem.MaxRetries)
		require.NotNil(t, j2.WorkItem.MaxRetries)
		assert.Equal(t, 20, *j2.WorkItem.MaxRetries)
	})

	t.Run("Missing Params", func(t *testing.T) {
		reqBody := `{"max_retries": 10}`
		req := httptest.NewRequest(http.MethodPut, "/jobs/max-retries", bytes.NewBufferString(reqBody))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
