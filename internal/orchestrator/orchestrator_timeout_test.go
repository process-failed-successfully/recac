package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateJobTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to pendingJobs, set RunAfter to ensure it stays pending
		orch.pendingJobs["job1"] = JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID:       "job1",
				Timeout:  10 * time.Minute,
				RunAfter: time.Now().Add(1 * time.Hour),
			},
		}

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.NoError(t, err)

		// Verify timeout was updated
		orch.mu.RLock()
		job := orch.pendingJobs["job1"]
		orch.mu.RUnlock()
		assert.Equal(t, 20*time.Minute, job.WorkItem.Timeout)
	})

	t.Run("JobNotFound", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		err := orch.UpdateJobTimeout(ctx, "missing-job", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in pending queue")
	})

	t.Run("JobAlreadyActive", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to activeJobs
		orch.activeJobs["job1"] = JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID: "job1",
			},
		}

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already active and cannot have timeout updated")
	})

	t.Run("JobAlreadyCompleted", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to completedJobs
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID: "job1",
			},
		})

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already completed")
	})
}
