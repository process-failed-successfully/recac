package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateJobPriority(t *testing.T) {
	ctx := context.Background()
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, 100*time.Millisecond)
	orch.MaxConcurrentJobs = 0 // unlimited

	// Setup a job in pendingJobs
	jobID := "TEST-1"
	orch.mu.Lock()
	orch.pendingJobs[jobID] = JobInfo{
		ID:        jobID,
		Summary:   "Test Pending",
		StartTime: time.Now(),
		Status:    "Pending",
		WorkItem: WorkItem{
			ID:        jobID,
			Priority:  0,
			DependsOn: []string{"dep1"}, // keep it pending
		},
	}
	orch.mu.Unlock()

	// 1. Update existing pending job
	err := orch.UpdateJobPriority(ctx, jobID, 10, nil)
	assert.NoError(t, err)

	orch.mu.RLock()
	job, exists := orch.pendingJobs[jobID]
	orch.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, 10, job.WorkItem.Priority)

	// 2. Update non-existent job
	err = orch.UpdateJobPriority(ctx, "NON-EXISTENT", 5, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in pending queue")

	// 3. Update active job
	activeID := "TEST-ACTIVE"
	orch.mu.Lock()
	orch.activeJobs[activeID] = JobInfo{
		ID:        activeID,
		Status:    "Spawning",
	}
	orch.mu.Unlock()

	err = orch.UpdateJobPriority(ctx, activeID, 5, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// 4. Update completed job
	completedID := "TEST-COMPLETED"
	orch.mu.Lock()
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     completedID,
		Status: "Completed",
	})
	orch.mu.Unlock()

	err = orch.UpdateJobPriority(ctx, completedID, 5, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}
