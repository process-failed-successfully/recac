package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestOrchestrator_ApproveJobsByTag(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	// Submit jobs
	orch.SubmitJob(context.Background(), WorkItem{ID: "J1", Tags: []string{"tag1"}}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "J2", Tags: []string{"tag1", "tag2"}}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "J3", Tags: []string{"tag3"}}, nil)

	// Verify all are pending approval
	for _, id := range []string{"J1", "J2", "J3"} {
		j, _ := orch.GetJob(id)
		if j.Status != "Pending Approval" {
			t.Fatalf("Job %s should be Pending Approval", id)
		}
	}

	// Approve by tag
	count, err := orch.ApproveJobsByTag(context.Background(), "tag1", nil)
	if err != nil {
		t.Fatalf("Failed to approve jobs by tag: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 jobs to be approved, got %d", count)
	}

	// Verify statuses
	j1, _ := orch.GetJob("J1")
	if !j1.Approved {
		t.Errorf("J1 should be approved")
	}

	j2, _ := orch.GetJob("J2")
	if !j2.Approved {
		t.Errorf("J2 should be approved")
	}

	j3, _ := orch.GetJob("J3")
	if j3.Approved {
		t.Errorf("J3 should NOT be approved")
	}
}

func TestOrchestrator_ApproveJobsByGroup(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	// Add a few jobs
	j1 := WorkItem{ID: "J1", ConcurrencyGroup: "group1"}
	j2 := WorkItem{ID: "J2", ConcurrencyGroup: "group2"}
	j3 := WorkItem{ID: "J3", ConcurrencyGroup: "group1"}

	_ = orch.SubmitJob(context.Background(), j1, nil)
	_ = orch.SubmitJob(context.Background(), j2, nil)
	_ = orch.SubmitJob(context.Background(), j3, nil)

	// Approve by Group
	count, err := orch.ApproveJobsByConcurrencyGroup(context.Background(), "group1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify statuses
	job1, _ := orch.GetJob("J1")
	assert.Equal(t, "Pending", job1.Status)
	assert.True(t, job1.Approved)

	job2, _ := orch.GetJob("J2")
	assert.Equal(t, "Pending Approval", job2.Status)
	assert.False(t, job2.Approved)

	job3, _ := orch.GetJob("J3")
	assert.Equal(t, "Pending", job3.Status)
	assert.True(t, job3.Approved)
}

func TestOrchestrator_ApproveJobsByMatch(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)
	orch.RequireApproval = true

	// Submit jobs
	orch.SubmitJob(context.Background(), WorkItem{ID: "FOO-1"}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "FOO-2"}, nil)
	orch.SubmitJob(context.Background(), WorkItem{ID: "BAR-1"}, nil)

	// Verify all are pending approval
	for _, id := range []string{"FOO-1", "FOO-2", "BAR-1"} {
		j, _ := orch.GetJob(id)
		if j.Status != "Pending Approval" {
			t.Fatalf("Job %s should be Pending Approval", id)
		}
	}

	// Approve by match
	count, err := orch.ApproveJobsByMatch(context.Background(), "FOO-.*", nil)
	if err != nil {
		t.Fatalf("Failed to approve jobs by match: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 jobs to be approved, got %d", count)
	}

	// Verify statuses
	f1, _ := orch.GetJob("FOO-1")
	if !f1.Approved {
		t.Errorf("FOO-1 should be approved")
	}

	f2, _ := orch.GetJob("FOO-2")
	if !f2.Approved {
		t.Errorf("FOO-2 should be approved")
	}

	b1, _ := orch.GetJob("BAR-1")
	if b1.Approved {
		t.Errorf("BAR-1 should NOT be approved")
	}
}
