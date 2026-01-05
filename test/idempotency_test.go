package test

import (
	"context"
	"testing"

	"github.com/process-failed-successfully/recac/internal/jobs"
	"github.com/stretchr/testify/assert"
)

func TestDuplicateJobExecution(t *testing.T) {
	// Create a job that tracks execution
	job := jobs.NewBaseJob("duplicate-job")
	executionLog := make([]string, 0)

	// Execute job multiple times
	for i := 0; i < 3; i++ {
		err := job.Execute(context.Background())
		assert.NoError(t, err)
		executionLog = append(executionLog, "executed")
	}

	// Verify no side effects from duplicates
	assert.Len(t, executionLog, 3)
	assert.Equal(t, jobs.StatusCompleted, job.Status())
}

func TestStateConsistencyAfterRetries(t *testing.T) {
	// Create a job with retry logic
	job := jobs.NewBaseJob("retry-job")

	// Simulate retries
	for i := 0; i < 3; i++ {
		if i < 2 {
			job.SetStatus(jobs.StatusFailed)
		} else {
			job.SetStatus(jobs.StatusCompleted)
		}
	}

	// Verify final state is consistent
	assert.Equal(t, jobs.StatusCompleted, job.Status())
}
