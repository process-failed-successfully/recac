package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

// simple wait loop
func waitForCondition(f func() bool) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for condition")
}

func TestOrchestratorRequireApproval(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	item := WorkItem{
		ID:      "JOB-1",
		Summary: "Test approval job",
	}

	ctx := context.Background()
	logger := slog.Default()

	// Initial submission
	err := orch.SubmitJob(ctx, item, logger)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Verify job is pending approval
	job, err := orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Job not found: %v", err)
	}
	if job.Status != "Pending Approval" {
		t.Errorf("Expected status 'Pending Approval', got '%s'", job.Status)
	}
	if job.Approved {
		t.Errorf("Expected Approved to be false, got true")
	}

	// Try to submit the same job again, should fail with pending approval error
	err = orch.SubmitJob(ctx, item, logger)
	if err == nil {
		t.Fatalf("Expected error when submitting duplicate pending approval job, got nil")
	}

	// Approve job
	err = orch.ApproveJob(ctx, item.ID, logger)
	if err != nil {
		t.Fatalf("Failed to approve job: %v", err)
	}

	// Wait for the job to complete
	if err := waitForCondition(func() bool {
		j, _ := orch.GetJob(item.ID)
		return j.Status == "Completed" || j.Status == "Failed"
	}); err != nil {
		t.Fatalf("Job did not finish: %v", err)
	}

	// Verify job was run
	job, err = orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Job not found: %v", err)
	}
	if job.Status != "Completed" {
		t.Errorf("Expected status 'Completed', got '%s'", job.Status)
	}
	if !job.Approved {
		t.Errorf("Expected Approved to be true, got false")
	}
}

func TestOrchestratorApproveJob_NotFound(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	err := orch.ApproveJob(context.Background(), "UNKNOWN", nil)
	if err == nil || err.Error() != "job UNKNOWN not found" {
		t.Errorf("Expected not found error, got %v", err)
	}
}

func TestOrchestratorApproveJob_NotPendingApproval(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	// Create a job that is just pending (e.g. dependencies), not pending approval
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:     "JOB-1",
		Status: "Pending",
	}

	err := orch.ApproveJob(context.Background(), "JOB-1", nil)
	if err == nil || err.Error() != "job JOB-1 is not pending approval" {
		t.Errorf("Expected not pending approval error, got %v", err)
	}
}
