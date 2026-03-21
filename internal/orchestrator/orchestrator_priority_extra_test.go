package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateJobPriorityWithPersistence(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	mp := &mockPersistence{}
	orch.SetPersistence(mp)

	ctx := context.Background()

	// 1. Job pending dependencies
	orch.mu.Lock()
	orch.pendingJobs["JOB-PENDING"] = JobInfo{
		ID:       "JOB-PENDING",
		WorkItem: WorkItem{ID: "JOB-PENDING", DependsOn: []string{"DEP"}, Priority: 1}, // Has unmet dep so it stays pending
	}
	orch.mu.Unlock()

	priority := 10

	// Test pending job update
	err := orch.UpdateJobPriority(ctx, "JOB-PENDING", priority, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job, ok := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()
	require.True(t, ok)
	assert.Equal(t, priority, job.WorkItem.Priority)

	require.Contains(t, mp.savedJobs, "JOB-PENDING")
	assert.Equal(t, priority, mp.savedJobs["JOB-PENDING"].WorkItem.Priority)
}

func TestUpdateJobsPriorityBulkWithPersistence(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	mp := &mockPersistence{}
	orch.SetPersistence(mp)

	ctx := context.Background()

	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID: "JOB-1",
			Summary: "Fix bug 1",
		WorkItem: WorkItem{
			ID:        "JOB-1",
			Tags:      []string{"backend"},
			Summary:   "Fix bug 1",
			DependsOn: []string{"DEP"}, // Needs dep to stay pending
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID: "JOB-2",
			Summary: "Fix bug 2",
		WorkItem: WorkItem{
			ID:        "JOB-2",
			Tags:      []string{"frontend"},
			Summary:   "Fix bug 2",
			DependsOn: []string{"DEP"},
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID: "JOB-3",
			Summary: "New feature",
		WorkItem: WorkItem{
			ID:        "JOB-3",
			Tags:      []string{"backend", "urgent"},
			Summary:   "New feature",
			DependsOn: []string{"DEP"},
		},
	}
	orch.mu.Unlock()

	priority := 15

	// Test UpdateJobsPriorityByTag
	count, err := orch.UpdateJobsPriorityByTag(ctx, "backend", priority, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	orch.mu.Lock()
	j1, ok1 := orch.pendingJobs["JOB-1"]
	j2, ok2 := orch.pendingJobs["JOB-2"]
	j3, ok3 := orch.pendingJobs["JOB-3"]
	orch.mu.Unlock()

	require.True(t, ok1)
	require.True(t, ok2)
	require.True(t, ok3)

	assert.Equal(t, priority, j1.WorkItem.Priority)
	assert.Equal(t, 0, j2.WorkItem.Priority)
	assert.Equal(t, priority, j3.WorkItem.Priority)

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-3")
	assert.NotContains(t, mp.savedJobs, "JOB-2")

	// Test UpdateJobsPriorityByMatch
	priorityMatch := 20
	countMatch, errMatch := orch.UpdateJobsPriorityByMatch(ctx, "Fix", priorityMatch, nil)
	require.NoError(t, errMatch)
	assert.Equal(t, 2, countMatch)

	orch.mu.Lock()
	j1, _ = orch.pendingJobs["JOB-1"]
	j2, _ = orch.pendingJobs["JOB-2"]
	j3, _ = orch.pendingJobs["JOB-3"]
	orch.mu.Unlock()

	assert.Equal(t, priorityMatch, j1.WorkItem.Priority)
	assert.Equal(t, priorityMatch, j2.WorkItem.Priority)
	assert.Equal(t, priority, j3.WorkItem.Priority) // Should not change

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-2")
}
