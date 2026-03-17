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
		ID:     activeID,
		Status: "Spawning",
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

func TestUpdateJobsPriorityByTag(t *testing.T) {
	o := New(nil, nil, 1*time.Minute)

	job1 := WorkItem{ID: "job1", Tags: []string{"backend"}, Priority: 1}
	job2 := WorkItem{ID: "job2", Tags: []string{"frontend", "backend"}, Priority: 1}
	job3 := WorkItem{ID: "job3", Tags: []string{"frontend"}, Priority: 1}

	o.pendingJobs = map[string]JobInfo{
		"job1": {ID: "job1", WorkItem: job1, RetryAfter: time.Now().Add(1 * time.Hour)},
		"job2": {ID: "job2", WorkItem: job2, RetryAfter: time.Now().Add(1 * time.Hour)},
		"job3": {ID: "job3", WorkItem: job3, RetryAfter: time.Now().Add(1 * time.Hour)},
	}

	count, err := o.UpdateJobsPriorityByTag(context.Background(), "backend", 5, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, 5, o.pendingJobs["job1"].WorkItem.Priority)
	assert.Equal(t, 5, o.pendingJobs["job2"].WorkItem.Priority)
	assert.Equal(t, 1, o.pendingJobs["job3"].WorkItem.Priority)
}

func TestUpdateJobsPriorityByMatch(t *testing.T) {
	o := New(nil, nil, 1*time.Minute)

	job1 := WorkItem{ID: "job1", Summary: "Fix backend bug", Priority: 1}
	job2 := WorkItem{ID: "job2", Summary: "Fix frontend bug", Priority: 1}
	job3 := WorkItem{ID: "job3", Summary: "Add new feature", Priority: 1}

	o.pendingJobs = map[string]JobInfo{
		"job1": {ID: "job1", WorkItem: job1, Summary: job1.Summary, RetryAfter: time.Now().Add(1 * time.Hour)},
		"job2": {ID: "job2", WorkItem: job2, Summary: job2.Summary, RetryAfter: time.Now().Add(1 * time.Hour)},
		"job3": {ID: "job3", WorkItem: job3, Summary: job3.Summary, RetryAfter: time.Now().Add(1 * time.Hour)},
	}

	count, err := o.UpdateJobsPriorityByMatch(context.Background(), "bug", 10, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, 10, o.pendingJobs["job1"].WorkItem.Priority)
	assert.Equal(t, 10, o.pendingJobs["job2"].WorkItem.Priority)
	assert.Equal(t, 1, o.pendingJobs["job3"].WorkItem.Priority)
}
