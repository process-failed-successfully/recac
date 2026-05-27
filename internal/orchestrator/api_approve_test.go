package orchestrator

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestAPI_ApproveBulkJobs(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	// Submit jobs
	orch.SubmitJob(context.Background(), WorkItem{ID: "J1", Tags: []string{"tag1"}}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "FOO-1"}, nil)

	// Submit an older job
	orch.SubmitJob(context.Background(), WorkItem{ID: "OLD-1"}, nil)
	orch.mu.Lock()
	oldJob := orch.pendingJobs["OLD-1"]
	oldJob.StartTime = time.Now().Add(-2 * time.Hour)
	orch.pendingJobs["OLD-1"] = oldJob
	orch.mu.Unlock()

	// Test Approve by Older Than
	req := httptest.NewRequest(http.MethodPost, "/jobs/approve?older_than=1h", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	old1, _ := orch.GetJob("OLD-1")
	if !old1.Approved {
		t.Errorf("OLD-1 should be approved")
	}

	// Test Approve by Tag
	req = httptest.NewRequest(http.MethodPost, "/jobs/approve?tag=tag1", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	j1, _ := orch.GetJob("J1")
	if !j1.Approved {
		t.Errorf("J1 should be approved")
	}

	// Test Approve by Match
	req = httptest.NewRequest(http.MethodPost, "/jobs/approve?match=FOO-.*", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	foo1, _ := orch.GetJob("FOO-1")
	if !foo1.Approved {
		t.Errorf("FOO-1 should be approved")
	}

	// Submit another job for group
	orch.SubmitJob(context.Background(), WorkItem{ID: "G1", ConcurrencyGroup: "group1"}, nil)

	// Test Approve by Group
	req = httptest.NewRequest(http.MethodPost, "/jobs/approve?group=group1", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	g1, _ := orch.GetJob("G1")
	if !g1.Approved {
		t.Errorf("G1 should be approved")
	}

	// Test Missing Parameters
	req = httptest.NewRequest(http.MethodPost, "/jobs/approve", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", rr.Code)
	}
}
