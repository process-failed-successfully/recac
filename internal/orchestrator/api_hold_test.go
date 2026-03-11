package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestAPI_HoldUnholdJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	orch.RequireApproval = true

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())

	server := httptest.NewServer(mux)
	defer server.Close()

	// 1. Submit a job that will be pending approval
	item := WorkItem{
		ID:      "TEST-API-HOLD",
		Summary: "Test API Hold",
	}
	err := orch.SubmitJob(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// 2. Hold via API
	resp, err := http.Post(server.URL+"/jobs/"+item.ID+"/hold", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to call hold API: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	job, _ := orch.GetJob(item.ID)
	if !job.WorkItem.Hold {
		t.Fatalf("Expected job to be held via API")
	}

	// 3. Unhold via API
	resp, err = http.Post(server.URL+"/jobs/"+item.ID+"/unhold", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to call unhold API: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	job, _ = orch.GetJob(item.ID)
	if job.WorkItem.Hold {
		t.Fatalf("Expected job to be unheld via API")
	}

	// 4. Test missing job
	resp, err = http.Post(server.URL+"/jobs/missing/hold", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to call hold API on missing job: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
