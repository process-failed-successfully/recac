package test

import (
	"context"
	"testing"
	"time"

	"github.com/process-failed-successfully/recac/internal/jobs"
	"github.com/process-failed-successfully/recac/internal/orchestrator"
	"github.com/process-failed-successfully/recac/internal/retry"
	"github.com/stretchr/testify/assert"
)

func TestJobRetryWithBackoff(t *testing.T) {
	// Create a job with retry configuration
	job := jobs.NewBaseJob("test-job")
	retryConfig := retry.NewConfig(3, 1*time.Second)

	// Simulate transient failures
	failures := 0
	retryLogic := retry.NewExponentialBackoff(retryConfig, func(ctx context.Context) error {
		failures++
		if failures < 3 {
			return assert.AnError
		}
		return nil
	})

	// Execute with retries
	err := retryLogic.Execute(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 3, failures)
}

func TestIdempotentJobExecution(t *testing.T) {
	// Create an idempotent job
	job := jobs.NewBaseJob("idempotent-job")
	executionCount := 0

	// Execute job multiple times
	for i := 0; i < 3; i++ {
		err := job.Execute(context.Background())
		assert.NoError(t, err)
		executionCount++
	}

	// Verify job state is consistent
	assert.Equal(t, jobs.StatusCompleted, job.Status())
	assert.Equal(t, 3, executionCount)
}

func TestOrphanAdoptionResilience(t *testing.T) {
	registry := orchestrator.NewJobRegistry()
	adopter := orchestrator.NewJobAdopter(registry)

	// Create an orphaned job
	orphanJob := jobs.NewBaseJob("orphan-job")
	orphanJob.SetStatus(jobs.StatusRunning)

	// Adopt the job
	err := adopter.AdoptJob(context.Background(), orphanJob)
	assert.NoError(t, err)

	// Verify job is adopted and reset
	adoptedJob, exists := registry.GetJob("orphan-job")
	assert.True(t, exists)
	assert.Equal(t, jobs.StatusPending, adoptedJob.Status())
}
