package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_HoldUnholdJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	// 1. Submit a job that will be held initially (e.g. requires approval)
	orch.RequireApproval = true
	item := WorkItem{
		ID:      "TEST-HOLD",
		Summary: "Test Hold Job",
	}

	err := orch.SubmitJob(ctx, item, nil)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait a tiny bit just in case
	time.Sleep(50 * time.Millisecond)

	// Verify it's pending approval
	job, err := orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if job.Status != "Pending Approval" {
		t.Fatalf("Expected job to be Pending Approval, got %s", job.Status)
	}

	// 2. Hold the job
	err = orch.HoldJob(ctx, item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to hold job: %v", err)
	}

	// Verify hold flag is set
	job, err = orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if !job.WorkItem.Hold {
		t.Fatalf("Expected job to be held")
	}

	// 3. Approve the job
	err = orch.ApproveJob(ctx, item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to approve job: %v", err)
	}

	// Wait to see if it schedules (it shouldn't because it's held)
	time.Sleep(50 * time.Millisecond)

	job, err = orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	// It should be Pending, not Spawning/Running, because it's held.
	if job.Status != "Pending" {
		t.Fatalf("Expected job to be Pending while held, got %s", job.Status)
	}
	if !job.WorkItem.Hold {
		t.Fatalf("Expected job to still be held")
	}

	// 4. Unhold the job
	err = orch.UnholdJob(ctx, item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to unhold job: %v", err)
	}

	// Wait for scheduler to pick it up
	time.Sleep(50 * time.Millisecond)

	// Verify it scheduled (completed or active)
	job, err = orch.GetJob(item.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if job.WorkItem.Hold {
		t.Fatalf("Expected job to be unheld")
	}
	if job.Status == "Pending" {
		t.Fatalf("Expected job to NOT be Pending after unhold, got %s", job.Status)
	}
}

func TestOrchestrator_HoldJob_Errors(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	// 1. Try holding a non-existent job
	err := orch.HoldJob(ctx, "NON-EXISTENT", nil)
	if err == nil {
		t.Fatalf("Expected error when holding non-existent job")
	}

	// 2. Submit a normal job (it runs immediately)
	item := WorkItem{
		ID:      "TEST-ACTIVE",
		Summary: "Test Active Job",
	}
	err = orch.SubmitJob(ctx, item, nil)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Let it complete or start

	// Try holding it
	err = orch.HoldJob(ctx, "TEST-ACTIVE", nil)
	if err == nil {
		t.Fatalf("Expected error when holding active/completed job")
	}
}
