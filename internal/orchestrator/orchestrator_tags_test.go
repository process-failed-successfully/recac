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

func TestUpdateJobsTagsByTag(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 10*time.Second)

	orch.pendingJobs["JOB-1"] = JobInfo{
		ID: "JOB-1",
		WorkItem: WorkItem{
			ID:   "JOB-1",
			Tags: []string{"backend", "urgent"},
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID: "JOB-2",
		WorkItem: WorkItem{
			ID:   "JOB-2",
			Tags: []string{"frontend"},
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID: "JOB-3",
		WorkItem: WorkItem{
			ID:   "JOB-3",
			Tags: []string{"backend", "low"},
		},
	}

	count, err := orch.UpdateJobsTagsByTag(context.Background(), "backend", []string{"api", "v2"}, nil)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	job1, _ := orch.GetJob("JOB-1")
	require.Equal(t, []string{"api", "v2"}, job1.WorkItem.Tags)

	job3, _ := orch.GetJob("JOB-3")
	require.Equal(t, []string{"api", "v2"}, job3.WorkItem.Tags)

	job2, _ := orch.GetJob("JOB-2")
	require.Equal(t, []string{"frontend"}, job2.WorkItem.Tags)
}

func TestUpdateJobsTagsByMatch(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 10*time.Second)

	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:      "JOB-1",
		Summary: "Fix bug in payment service",
		WorkItem: WorkItem{
			ID:   "JOB-1",
			Tags: []string{"old"},
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID:      "JOB-2",
		Summary: "Update UI",
		WorkItem: WorkItem{
			ID:   "JOB-2",
			Tags: []string{"old"},
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID:    "JOB-3",
		Error: "Payment gateway timeout",
		WorkItem: WorkItem{
			ID:   "JOB-3",
			Tags: []string{"old"},
		},
	}

	count, err := orch.UpdateJobsTagsByMatch(context.Background(), "payment", []string{"payment-team"}, nil)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	job1, _ := orch.GetJob("JOB-1")
	require.Equal(t, []string{"payment-team"}, job1.WorkItem.Tags)

	job3, _ := orch.GetJob("JOB-3")
	require.Equal(t, []string{"payment-team"}, job3.WorkItem.Tags)

	job2, _ := orch.GetJob("JOB-2")
	require.Equal(t, []string{"old"}, job2.WorkItem.Tags)
}
