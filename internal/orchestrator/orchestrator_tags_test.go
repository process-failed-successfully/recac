package orchestrator

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestUpdateJobTagsWithPersistence(t *testing.T) {
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
		WorkItem: WorkItem{ID: "JOB-PENDING", DependsOn: []string{"DEP"}, Tags: []string{"tag1"}}, // Has unmet dep so it stays pending
	}
	orch.mu.Unlock()

	tags := []string{"tag2", "tag3"}

	// Test pending job update
	err := orch.UpdateJobTags(ctx, "JOB-PENDING", tags, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job, ok := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()
	require.True(t, ok)
	assert.Equal(t, tags, job.WorkItem.Tags)

	require.Contains(t, mp.savedJobs, "JOB-PENDING")
	assert.Equal(t, tags, mp.savedJobs["JOB-PENDING"].WorkItem.Tags)
}

func TestUpdateJobsTagsBulkWithPersistence(t *testing.T) {
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

	tags := []string{"new-backend"}

	// Test UpdateJobsTagsByTag
	count, err := orch.UpdateJobsTagsByTag(ctx, "backend", tags, nil)
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

	assert.Equal(t, tags, j1.WorkItem.Tags)
	assert.Equal(t, []string{"frontend"}, j2.WorkItem.Tags)
	assert.Equal(t, tags, j3.WorkItem.Tags)

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-3")
	assert.NotContains(t, mp.savedJobs, "JOB-2")

	// Test UpdateJobsTagsByMatch
	tagsMatch := []string{"fixed"}
	countMatch, errMatch := orch.UpdateJobsTagsByMatch(ctx, "Fix", tagsMatch, nil)
	require.NoError(t, errMatch)
	assert.Equal(t, 2, countMatch)

	orch.mu.Lock()
	j1, _ = orch.pendingJobs["JOB-1"]
	j2, _ = orch.pendingJobs["JOB-2"]
	j3, _ = orch.pendingJobs["JOB-3"]
	orch.mu.Unlock()

	assert.Equal(t, tagsMatch, j1.WorkItem.Tags)
	assert.Equal(t, tagsMatch, j2.WorkItem.Tags)
	assert.Equal(t, tags, j3.WorkItem.Tags) // Should not change

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-2")
}

func TestOrchestrator_AddJobTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	ctx := context.Background()

	orch.mu.Lock()
	orch.pendingJobs["JOB-PENDING"] = JobInfo{
		ID: "JOB-PENDING",
		WorkItem: WorkItem{
			Tags: []string{"tag1"},
		},
	}
	orch.mu.Unlock()

	tagsToAdd := []string{"tag2", "tag2"} // Add tag2, test duplication
	err := orch.AddJobTags(ctx, "JOB-PENDING", tagsToAdd, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()

	assert.ElementsMatch(t, []string{"tag1", "tag2"}, job.WorkItem.Tags)
}

func TestOrchestrator_RemoveJobTags(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	ctx := context.Background()

	orch.mu.Lock()
	orch.pendingJobs["JOB-PENDING"] = JobInfo{
		ID: "JOB-PENDING",
		WorkItem: WorkItem{
			Tags: []string{"tag1", "tag2", "tag3"},
		},
	}
	orch.mu.Unlock()

	tagsToRemove := []string{"tag1", "tag3", "tag-not-exist"}
	err := orch.RemoveJobTags(ctx, "JOB-PENDING", tagsToRemove, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()

	assert.ElementsMatch(t, []string{"tag2"}, job.WorkItem.Tags)
}

func TestOrchestrator_AddJobTags_Errors(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	ctx := context.Background()

	// Setup state
	orch.mu.Lock()
	orch.activeJobs["JOB-ACTIVE"] = JobInfo{ID: "JOB-ACTIVE"}
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "JOB-COMPLETED"})
	orch.mu.Unlock()

	// Test adding to active job
	err := orch.AddJobTags(ctx, "JOB-ACTIVE", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// Test adding to completed job
	err = orch.AddJobTags(ctx, "JOB-COMPLETED", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")

	// Test adding to missing job
	err = orch.AddJobTags(ctx, "JOB-MISSING", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrchestrator_RemoveJobTags_Errors(t *testing.T) {
	orch := New(nil, nil, time.Minute)
	ctx := context.Background()

	// Setup state
	orch.mu.Lock()
	orch.activeJobs["JOB-ACTIVE"] = JobInfo{ID: "JOB-ACTIVE"}
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "JOB-COMPLETED"})
	orch.mu.Unlock()

	// Test removing from active job
	err := orch.RemoveJobTags(ctx, "JOB-ACTIVE", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// Test removing from completed job
	err = orch.RemoveJobTags(ctx, "JOB-COMPLETED", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")

	// Test removing from missing job
	err = orch.RemoveJobTags(ctx, "JOB-MISSING", []string{"tag1"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
