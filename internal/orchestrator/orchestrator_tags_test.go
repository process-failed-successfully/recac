package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateJobTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	// Setup a pending job
	job := JobInfo{
		ID:     "JOB-1",
		Status: "Pending",
		WorkItem: WorkItem{
			ID: "JOB-1",
		},
	}
	orch.pendingJobs["JOB-1"] = job

	// Setup an active job
	activeJob := JobInfo{
		ID:     "JOB-2",
		Status: "Running",
	}
	orch.activeJobs["JOB-2"] = activeJob

	// Setup a completed job
	completedJob := JobInfo{
		ID:     "JOB-3",
		Status: "Completed",
	}
	orch.completedJobs = append(orch.completedJobs, completedJob)

	ctx := context.Background()

	t.Run("Success_PendingQueue", func(t *testing.T) {
		tags := []string{"bug", "critical"}
		err := orch.UpdateJobTags(ctx, "JOB-1", tags, nil)
		require.NoError(t, err)

		updatedJob, ok := orch.pendingJobs["JOB-1"]
		require.True(t, ok)
		assert.Equal(t, tags, updatedJob.WorkItem.Tags)
	})

	t.Run("Error_ActiveJob", func(t *testing.T) {
		tags := []string{"feature"}
		err := orch.UpdateJobTags(ctx, "JOB-2", tags, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already active")
	})

	t.Run("Error_CompletedJob", func(t *testing.T) {
		tags := []string{"enhancement"}
		err := orch.UpdateJobTags(ctx, "JOB-3", tags, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already completed")
	})

	t.Run("Error_NotFound", func(t *testing.T) {
		tags := []string{"test"}
		err := orch.UpdateJobTags(ctx, "NON-EXISTENT", tags, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
