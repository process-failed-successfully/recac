package orchestrator

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPI_ApproveJob(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	// Try to approve a non-existent job
	req := httptest.NewRequest(http.MethodPost, "/jobs/UNKNOWN/approve", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found, got %d", rr.Code)
	}

	// Create a job pending approval
	item := WorkItem{ID: "JOB-1"}
	_ = orch.SubmitJob(context.Background(), item, nil)

	req = httptest.NewRequest(http.MethodPost, "/jobs/JOB-1/approve", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	// Wait for the job to complete
	_ = waitForCondition(func() bool {
		j, _ := orch.GetJob(item.ID)
		return j.Status == "Completed" || j.Status == "Failed"
	})

	// Try to approve again, should not be pending approval
	req = httptest.NewRequest(http.MethodPost, "/jobs/JOB-1/approve", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", rr.Code)
	}
}
