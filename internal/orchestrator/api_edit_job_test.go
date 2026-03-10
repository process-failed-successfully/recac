package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_EditJob(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 1*time.Minute)
	logger := slog.Default()

	// Add a pending job
	item := WorkItem{
		ID:      "JOB-1",
		Summary: "Initial Summary",
	}
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:       "JOB-1",
		Summary:  item.Summary,
		Status:   "Pending",
		WorkItem: item,
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	t.Run("Valid Update", func(t *testing.T) {
		newItem := WorkItem{
			ID:      "JOB-1",
			Summary: "Updated Summary",
			EnvVars: map[string]string{"K": "V"},
		}
		body, _ := json.Marshal(newItem)
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "work item updated")

		updatedJob, err := orch.GetJob("JOB-1")
		assert.NoError(t, err)
		assert.Equal(t, "Updated Summary", updatedJob.Summary)
		assert.Equal(t, "V", updatedJob.WorkItem.EnvVars["K"])
	})

	t.Run("Invalid ID in Body", func(t *testing.T) {
		newItem := WorkItem{
			ID:      "JOB-2", // Does not match URL
			Summary: "Updated Summary",
		}
		body, _ := json.Marshal(newItem)
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-1", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "must match URL")
	})

	t.Run("Active Job Update", func(t *testing.T) {
		// Make it active
		orch.mu.Lock()
		orch.activeJobs["JOB-3"] = JobInfo{ID: "JOB-3", Status: "Running"}
		orch.mu.Unlock()

		newItem := WorkItem{ID: "JOB-3"}
		body, _ := json.Marshal(newItem)
		req := httptest.NewRequest(http.MethodPut, "/jobs/JOB-3", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		assert.Contains(t, rr.Body.String(), "already active")
	})
}
