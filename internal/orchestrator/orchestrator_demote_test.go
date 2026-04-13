package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDemoteJob(t *testing.T) {
	poller := newMockPoller([]WorkItem{})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	// Add some pending jobs
	orch.pendingJobs["job1"] = JobInfo{
		ID:       "job1",
		Status:   "Pending",
		WorkItem: WorkItem{Priority: 10},
	}
	orch.pendingJobs["job2"] = JobInfo{
		ID:       "job2",
		Status:   "Pending",
		WorkItem: WorkItem{Priority: 5},
	}
	orch.pendingJobs["job3"] = JobInfo{
		ID:       "job3",
		Status:   "Pending",
		WorkItem: WorkItem{Priority: 1}, // Currently min priority
	}

	// Demote job1
	newPriority, err := orch.DemoteJob(context.Background(), "job1", silentLogger)
	assert.NoError(t, err)

	// Min was 1, so new priority should be 0
	assert.Equal(t, 0, newPriority)
	assert.Equal(t, 0, orch.pendingJobs["job1"].WorkItem.Priority)

	// Demote job2, min is now 0, new priority should be -1
	newPriority, err = orch.DemoteJob(context.Background(), "job2", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, -1, newPriority)
	assert.Equal(t, -1, orch.pendingJobs["job2"].WorkItem.Priority)

	// Demote non-existent job
	_, err = orch.DemoteJob(context.Background(), "job_missing", silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDemoteJob_EmptyOtherJobs(t *testing.T) {
	poller := newMockPoller([]WorkItem{})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	orch.pendingJobs["job1"] = JobInfo{
		ID:       "job1",
		Status:   "Pending",
		WorkItem: WorkItem{Priority: 10},
	}

	// Only 1 job in queue, min defaults to 0 from the loop initialization logic (because first remains true),
	// so minPriority is 0. newPriority is 0 - 1 = -1. But since it checks if newPriority >= job.WorkItem.Priority
	// (-1 >= 10 is false), it assigns newPriority = minPriority - 1 = -1. Wait, actually the logic in DemoteJob is:
	// newPriority := minPriority - 1
	// if newPriority >= job.WorkItem.Priority { newPriority = job.WorkItem.Priority - 1 }
	// So -1 >= 10 is false. It stays -1.
	newPriority, err := orch.DemoteJob(context.Background(), "job1", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, -1, newPriority)
	assert.Equal(t, -1, orch.pendingJobs["job1"].WorkItem.Priority)
}
