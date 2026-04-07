package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPI_FailJob(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Second)

	orch.pendingJobs["job123"] = JobInfo{
		ID:     "job123",
		Status: "Pending",
		WorkItem: WorkItem{
			ID: "job123",
			Tags: []string{"tag1"},
		},
	}

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())

	t.Run("Fail single job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/jobs/job123/fail", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		if _, exists := orch.pendingJobs["job123"]; exists {
			t.Errorf("job should have been removed from pending jobs")
		}
	})

	// Re-add for bulk
	orch.pendingJobs["job456"] = JobInfo{
		ID:     "job456",
		Status: "Pending",
		WorkItem: WorkItem{
			ID: "job456",
			Tags: []string{"tag1"},
		},
	}

	t.Run("Fail bulk jobs by tag", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/jobs/fail?tag=tag1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		if _, exists := orch.pendingJobs["job456"]; exists {
			t.Errorf("job should have been removed from pending jobs")
		}
	})

	t.Run("Fail bulk without query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/jobs/fail", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}
