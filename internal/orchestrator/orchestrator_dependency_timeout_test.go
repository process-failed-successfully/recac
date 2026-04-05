package orchestrator

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestDependencyTimeout(t *testing.T) {
	t.Run("job fails when dependency timeout is exceeded", func(t *testing.T) {
		poller := &MockPoller{}
		spawner := &MockSpawner{}
		orch := New(poller, spawner, 1*time.Minute)

		dt := 1 * time.Second
		item := WorkItem{
			ID:                "job-1",
			DependsOn:         []string{"job-missing"},
			DependencyTimeout: &dt,
		}

		ctx := context.Background()
		logger := slog.Default()

		// Submit the job, it will be pending because dependency "job-missing" is missing
		err := orch.SubmitJob(ctx, item, logger)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify it's pending
		pendingJobs := orch.GetPendingJobs()
		if len(pendingJobs) != 1 {
			t.Fatalf("expected 1 pending job, got %d", len(pendingJobs))
		}

		// Artificially age the job
		orch.mu.Lock()
		job := orch.pendingJobs["job-1"]
		job.StartTime = time.Now().Add(-2 * time.Second) // Older than DependencyTimeout
		orch.pendingJobs["job-1"] = job
		orch.mu.Unlock()

		// Trigger evaluation
		orch.evaluatePendingJobs(ctx, logger)

		// Verify it was removed from pending
		pendingJobs = orch.GetPendingJobs()
		if len(pendingJobs) != 0 {
			t.Fatalf("expected 0 pending jobs, got %d", len(pendingJobs))
		}

		// Verify it is in history with failed status
		completedJobs := orch.GetCompletedJobs()
		if len(completedJobs) != 1 {
			t.Fatalf("expected 1 completed job, got %d", len(completedJobs))
		}

		if completedJobs[0].Status != "Failed" {
			t.Errorf("expected job status Failed, got %s", completedJobs[0].Status)
		}

		if completedJobs[0].Error != "Dependency wait timeout exceeded" {
			t.Errorf("expected error 'Dependency wait timeout exceeded', got %s", completedJobs[0].Error)
		}
	})

	t.Run("job remains pending when dependency timeout is not exceeded", func(t *testing.T) {
		poller := &MockPoller{}
		spawner := &MockSpawner{}
		orch := New(poller, spawner, 1*time.Minute)

		dt := 10 * time.Second
		item := WorkItem{
			ID:                "job-2",
			DependsOn:         []string{"job-missing"},
			DependencyTimeout: &dt,
		}

		ctx := context.Background()
		logger := slog.Default()

		err := orch.SubmitJob(ctx, item, logger)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Trigger evaluation (time has not exceeded)
		orch.evaluatePendingJobs(ctx, logger)

		pendingJobs := orch.GetPendingJobs()
		if len(pendingJobs) != 1 {
			t.Fatalf("expected 1 pending job, got %d", len(pendingJobs))
		}

		completedJobs := orch.GetCompletedJobs()
		if len(completedJobs) != 0 {
			t.Fatalf("expected 0 completed jobs, got %d", len(completedJobs))
		}
	})
}
