package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func boolPtrExt(b bool) *bool {
	return &b
}

func durPtr(d time.Duration) *time.Duration {
	return &d
}

func floatPtr(f float64) *float64 {
	return &f
}

func TestOrchestrator_JobExecutionControl_RequireApproval(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
	orch.RequireApproval = false // Global approval is false

	// Job specifies it requires approval
	item := WorkItem{
		ID:              "job-approval-1",
		Summary:         "Requires Approval",
		RequireApproval: boolPtrExt(true),
	}

	ctx := context.Background()

	err := orch.SubmitJob(ctx, item, nil)
	assert.NoError(t, err)

	// Verify job is pending approval
	job, err := orch.GetJob("job-approval-1")
	assert.NoError(t, err)
	assert.Equal(t, "Pending Approval", job.Status)
	assert.False(t, job.Approved)

	// Approve it
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
		return i.ID == "job-approval-1"
	})).Return(nil).Once()

	err = orch.ApproveJob(ctx, "job-approval-1", nil)
	assert.NoError(t, err)

	// Since evaluatePendingJobs is async here but triggered synchronously inside ApproveJob, it should move to Spawning.
	job, _ = orch.GetJob("job-approval-1")
	assert.Equal(t, "Spawning", job.Status)
}

func TestOrchestrator_JobExecutionControl_RetryBackoff(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	maxRetries := 3
	item := WorkItem{
		ID:                     "job-backoff-1",
		Summary:                "Backoff Job",
		MaxRetries:             &maxRetries,
		RetryDelay:             durPtr(100 * time.Millisecond),
		RetryBackoffMultiplier: floatPtr(2.0),
	}

	ctx := context.Background()

	// Initial spawn fails
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
		return i.ID == "job-backoff-1"
	})).Return(assert.AnError).Once()
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	err := orch.SubmitJob(ctx, item, nil)
	assert.NoError(t, err)

	// Wait for failure and retry schedule
	time.Sleep(50 * time.Millisecond)

	job, err := orch.GetJob("job-backoff-1")
	assert.NoError(t, err)
	assert.Equal(t, "Retrying", job.Status)
	assert.Equal(t, 1, job.RetryCount)

	// Delay should be 100ms.
	expectedRetryTime := time.Now().Add(50 * time.Millisecond) // approximate
	assert.WithinDuration(t, expectedRetryTime, job.RetryAfter, 100*time.Millisecond)

	// Wait for retry to trigger.
	// Make spawn fail again.
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
		return i.ID == "job-backoff-1"
	})).Return(assert.AnError).Once()
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	time.Sleep(120 * time.Millisecond) // enough for first delay to pass

	job, _ = orch.GetJob("job-backoff-1")
	// If it failed and scheduled next retry, it will be "Retrying" again
	assert.Equal(t, "Retrying", job.Status)
	assert.Equal(t, 2, job.RetryCount)

	// Delay should now be 200ms (100ms * 2.0)
	// We check if it is roughly 200ms in the future from now
	assert.WithinDuration(t, time.Now().Add(200*time.Millisecond), job.RetryAfter, 100*time.Millisecond)

	// Wait for next retry and pass
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
		return i.ID == "job-backoff-1"
	})).Return(nil).Once()

	time.Sleep(250 * time.Millisecond)

	job, _ = orch.GetJob("job-backoff-1")
	assert.Equal(t, "Completed", job.Status)
}
