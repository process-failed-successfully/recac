package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestOrchestrator_FailJob(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 1*time.Second)

	// Setup pending job
	orch.pendingJobs["job-pending"] = JobInfo{
		ID:     "job-pending",
		Status: "Pending",
		WorkItem: WorkItem{
			ID: "job-pending",
		},
	}

	// Setup active job
	orch.activeJobs["job-active"] = JobInfo{
		ID:     "job-active",
		Status: "Active",
		WorkItem: WorkItem{
			ID: "job-active",
		},
	}
	orch.activeSpawns = 1

	// Setup completed job (to test we can't fail a completed job)
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "job-completed",
		Status: "Completed",
	})

	t.Run("Fail pending job", func(t *testing.T) {
		err := orch.FailJob(context.Background(), "job-pending", nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if _, exists := orch.pendingJobs["job-pending"]; exists {
			t.Errorf("job should have been removed from pending queue")
		}

		found := false
		for _, job := range orch.completedJobs {
			if job.ID == "job-pending" {
				found = true
				if job.Status != "Failed" {
					t.Errorf("expected status Failed, got %s", job.Status)
				}
				if job.Error != "Job manually failed" {
					t.Errorf("expected error message 'Job manually failed', got %s", job.Error)
				}
				break
			}
		}
		if !found {
			t.Errorf("job not found in history")
		}
	})

	t.Run("Fail active job", func(t *testing.T) {
		// Mock out Cancel since the fail method calls it when the job is active
		mockSpawner := orch.Spawner.(*MockSpawner)
		mockSpawner.On("Cancel", context.Background(), "job-active").Return(nil)

		err := orch.FailJob(context.Background(), "job-active", nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if _, exists := orch.activeJobs["job-active"]; exists {
			t.Errorf("job should have been removed from active jobs")
		}

		found := false
		for _, job := range orch.completedJobs {
			if job.ID == "job-active" {
				found = true
				if job.Status != "Failed" {
					t.Errorf("expected status Failed, got %s", job.Status)
				}
				if job.Error != "Job manually failed" {
					t.Errorf("expected error message 'Job manually failed', got %s", job.Error)
				}
				break
			}
		}
		if !found {
			t.Errorf("job not found in history")
		}
	})

	t.Run("Fail completed job (should fail)", func(t *testing.T) {
		err := orch.FailJob(context.Background(), "job-completed", nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("Fail non-existent job", func(t *testing.T) {
		err := orch.FailJob(context.Background(), "does-not-exist", nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
