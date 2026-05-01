package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_RetryFailedJobs(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	// Manually populate history
	orch.completedJobs = []JobInfo{
		{
			ID:       "JOB-1",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-1", Summary: "Failed Job 1"},
		},
		{
			ID:       "JOB-2",
			Status:   "Completed",
			WorkItem: WorkItem{ID: "JOB-2", Summary: "Success Job 2"},
		},
		{
			ID:       "JOB-3",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-3", Summary: "Failed Job 3"},
		},
	}

	ctx := context.Background()
	count, err := orch.RetryFailedJobs(ctx, "", "", "", silentLogger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Wait for async spawn to happen
	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	assert.Len(t, spawner.spawned, 2)

	ids := make(map[string]bool)
	for _, item := range spawner.spawned {
		ids[item.ID] = true
	}
	assert.True(t, ids["JOB-1"])
	assert.True(t, ids["JOB-3"])
	assert.False(t, ids["JOB-2"])
}

func TestOrchestrator_RetryFailedJobs_AlreadyActive(t *testing.T) {
	// If a job is failed in history but currently running (maybe manually submitted), it should not be retried.
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	// Add to active jobs
	orch.activeJobs["JOB-1"] = JobInfo{ID: "JOB-1", Status: "Running"}

	// Add to history as failed (maybe from a previous run)
	orch.completedJobs = []JobInfo{
		{
			ID:       "JOB-1",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-1", Summary: "Failed Job 1"},
		},
	}

	ctx := context.Background()
	count, err := orch.RetryFailedJobs(ctx, "", "", "", silentLogger)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	time.Sleep(10 * time.Millisecond)
	spawner.mu.Lock()
	assert.Empty(t, spawner.spawned)
	spawner.mu.Unlock()
}

func TestOrchestrator_RetryFailedJobs_WithMatch(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	// Manually populate history
	orch.completedJobs = []JobInfo{
		{
			ID:       "JOB-1",
			Status:   "Failed",
			Error:    "connection refused",
			WorkItem: WorkItem{ID: "JOB-1", Summary: "Failed Job 1"},
		},
		{
			ID:       "JOB-2",
			Status:   "Completed",
			WorkItem: WorkItem{ID: "JOB-2", Summary: "Success Job 2"},
		},
		{
			ID:       "JOB-3",
			Status:   "Failed",
			Error:    "timeout waiting for response",
			WorkItem: WorkItem{ID: "JOB-3", Summary: "Failed Job 3"},
		},
		{
			ID:       "JOB-4",
			Status:   "Failed",
			Error:    "database connection refused",
			WorkItem: WorkItem{ID: "JOB-4", Summary: "Failed Job 4"},
		},
	}

	ctx := context.Background()

	// Test matching "connection refused"
	count, err := orch.RetryFailedJobs(ctx, "connection refused", "", "", silentLogger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	assert.Len(t, spawner.spawned, 2)

	ids := make(map[string]bool)
	for _, item := range spawner.spawned {
		ids[item.ID] = true
	}
	assert.True(t, ids["JOB-1"])
	assert.False(t, ids["JOB-2"])
	assert.False(t, ids["JOB-3"])
	assert.True(t, ids["JOB-4"])
}

func TestOrchestrator_RetryFailedJobs_WithInvalidRegex(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Test with invalid regex
	count, err := orch.RetryFailedJobs(ctx, "[invalid regex", "", "", silentLogger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid retry match pattern")
	assert.Equal(t, 0, count)
}

func TestOrchestrator_RetryFailedJobs_WithTag(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	orch.completedJobs = []JobInfo{
		{
			ID:       "JOB-1",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-1", Summary: "Failed Job 1", Tags: []string{"backend"}},
		},
		{
			ID:       "JOB-2",
			Status:   "Completed",
			WorkItem: WorkItem{ID: "JOB-2", Summary: "Success Job 2", Tags: []string{"backend"}},
		},
		{
			ID:       "JOB-3",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-3", Summary: "Failed Job 3", Tags: []string{"frontend"}},
		},
		{
			ID:       "JOB-4",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-4", Summary: "Failed Job 4", Tags: []string{"BACKEND", "urgent"}},
		},
	}

	ctx := context.Background()

	// Test matching tag "backend"
	count, err := orch.RetryFailedJobs(ctx, "", "backend", "", silentLogger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	assert.Len(t, spawner.spawned, 2)

	ids := make(map[string]bool)
	for _, item := range spawner.spawned {
		ids[item.ID] = true
	}
	assert.True(t, ids["JOB-1"])
	assert.False(t, ids["JOB-2"]) // Completed, not failed
	assert.False(t, ids["JOB-3"]) // No backend tag
	assert.True(t, ids["JOB-4"])  // Has BACKEND tag (case-insensitive check)
}

func TestOrchestrator_RetryFailedJobs_WithGroup(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	orch.completedJobs = []JobInfo{
		{
			ID:       "JOB-1",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-1", Summary: "Failed Job 1", ConcurrencyGroup: "groupA"},
		},
		{
			ID:       "JOB-2",
			Status:   "Completed",
			WorkItem: WorkItem{ID: "JOB-2", Summary: "Success Job 2", ConcurrencyGroup: "groupA"},
		},
		{
			ID:       "JOB-3",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-3", Summary: "Failed Job 3", ConcurrencyGroup: "groupB"},
		},
		{
			ID:       "JOB-4",
			Status:   "Failed",
			WorkItem: WorkItem{ID: "JOB-4", Summary: "Failed Job 4", ConcurrencyGroup: "GROUPa"},
		},
	}

	ctx := context.Background()

	// Test matching group "groupA"
	count, err := orch.RetryFailedJobs(ctx, "", "", "groupA", silentLogger)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	assert.Len(t, spawner.spawned, 2)

	ids := make(map[string]bool)
	for _, item := range spawner.spawned {
		ids[item.ID] = true
	}
	assert.True(t, ids["JOB-1"])
	assert.False(t, ids["JOB-2"]) // Completed, not failed
	assert.False(t, ids["JOB-3"]) // Not in groupA
	assert.True(t, ids["JOB-4"])  // Has GROUPa group (case-insensitive check)
}
