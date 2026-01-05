package test

import (
	"context"
	"testing"
	"time"

	"github.com/process-failed-successfully/recac/internal/jobs"
	"github.com/stretchr/testify/assert"
)

func TestBaseJobIdempotency(t *testing.T) {
	t.Run("Execute returns false for already completed job", func(t *testing.T) {
		job := jobs.NewBaseJob("test-1")

		// First execution
		executed, err := job.Execute(context.Background())
		assert.NoError(t, err)
		assert.False(t, executed) // BaseJob.executeLogic returns false by default

		// Mark as completed
		err = job.SetStatus(jobs.StatusCompleted)
		assert.NoError(t, err)

		// Second execution should be a no-op
		executed, err = job.Execute(context.Background())
		assert.NoError(t, err)
		assert.False(t, executed)
	})

	t.Run("Cannot update status of completed job", func(t *testing.T) {
		job := jobs.NewBaseJob("test-2")

		// Complete the job
		err := job.SetStatus(jobs.StatusCompleted)
		assert.NoError(t, err)

		// Try to update status
		err = job.SetStatus(jobs.StatusRunning)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot update status of completed or failed job")
	})
}

func TestSampleJobIdempotency(t *testing.T) {
	t.Run("SampleJob executes only once", func(t *testing.T) {
		job := jobs.NewSampleJob("sample-1", "test-operation")

		// First execution
		executed, err := job.Execute(context.Background())
		assert.NoError(t, err)
		assert.True(t, executed)
		assert.Equal(t, "Processed: test-operation", job.Result())
		assert.Equal(t, jobs.StatusCompleted, job.Status())

		// Second execution should be a no-op
		executed, err = job.Execute(context.Background())
		assert.NoError(t, err)
		assert.False(t, executed) // Already completed, so returns false
		assert.Equal(t, "Processed: test-operation", job.Result())
	})

	t.Run("SampleJob handles context cancellation", func(t *testing.T) {
		job := jobs.NewSampleJob("sample-2", "cancelled-operation")

		// Create a context that cancels immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		executed, err := job.Execute(ctx)
		assert.Error(t, err)
		assert.False(t, executed)
		assert.Equal(t, "", job.Result())
	})
}

func TestJobStatusTransitions(t *testing.T) {
	t.Run("Job status transitions correctly", func(t *testing.T) {
		job := jobs.NewBaseJob("status-test")

		assert.Equal(t, jobs.StatusPending, job.Status())

		// Execute should transition to running then completed
		_, err := job.Execute(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, jobs.StatusCompleted, job.Status())

		// Cannot transition from completed
		err = job.SetStatus(jobs.StatusRunning)
		assert.Error(t, err)
	})

	t.Run("Failed job cannot be re-executed", func(t *testing.T) {
		// Create a mock job that always fails
		job := &struct {
			*jobs.BaseJob
		}{
			BaseJob: jobs.NewBaseJob("fail-test"),
		}

		// Override executeLogic to fail
		originalExecuteLogic := job.Execute
		job.Execute = func(ctx context.Context) (bool, error) {
			// Simulate failure
			return false, assert.AnError
		}

		_, err := job.Execute(context.Background())
		assert.Error(t, err)

		// Job should be in failed state
		assert.Equal(t, jobs.StatusFailed, job.Status())

		// Cannot execute failed job
		_, err = job.Execute(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "job previously failed")
	})
}
