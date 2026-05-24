package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_SkipJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	// Submit a job that will remain pending because of RequireApproval
	item := WorkItem{ID: "TEST-SKIP", Summary: "To be skipped"}
	err := orch.SubmitJob(ctx, item, nil)
	assert.NoError(t, err)

	// Skip it
	err = orch.SkipJob(ctx, "TEST-SKIP", nil)
	assert.NoError(t, err)

	// Verify it's in history as Skipped
	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "TEST-SKIP", completed[0].ID)
	assert.Equal(t, "Skipped", completed[0].Status)
	assert.Equal(t, "Skipped by user", completed[0].Error)

	// Test skip non-existent
	err = orch.SkipJob(ctx, "NON-EXISTENT", nil)
	assert.Error(t, err)
}

func TestOrchestrator_SkipJobsByTag(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "S1", Tags: []string{"tag1"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J2", Summary: "S2", Tags: []string{"tag1", "tag2"}}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J3", Summary: "S3", Tags: []string{"tag3"}}, nil)

	count, err := orch.SkipJobsByTag(ctx, "tag1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	for _, j := range completed {
		assert.Equal(t, "Skipped", j.Status)
	}

	pending := orch.GetPendingJobs()
	assert.Len(t, pending, 1)
	assert.Equal(t, "J3", pending[0].ID)
}

func TestOrchestrator_SkipJobsByGroup(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", ConcurrencyGroup: "group1"}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J2", ConcurrencyGroup: "group2"}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J3", ConcurrencyGroup: "group1"}, nil)

	count, err := orch.SkipJobsByGroup(ctx, "group1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "Skipped", completed[0].Status)
	assert.Equal(t, "Skipped", completed[1].Status)

	pending := orch.GetPendingJobs()
	assert.Len(t, pending, 1)
	assert.Equal(t, "J2", pending[0].ID)
}

func TestOrchestrator_SkipJobsByMatch(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "J1", Summary: "Matches Foo"}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J2", Summary: "Matches Bar"}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "J3", Summary: "Matches foo again"}, nil)

	count, err := orch.SkipJobsByMatch(ctx, "foo", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	for _, j := range completed {
		assert.Equal(t, "Skipped", j.Status)
	}

	pending := orch.GetPendingJobs()
	assert.Len(t, pending, 1)
	assert.Equal(t, "J2", pending[0].ID)
}

func TestOrchestrator_DependencyMetOnSkip(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true
	ctx := context.Background()

	orch.SubmitJob(ctx, WorkItem{ID: "DEP1", Summary: "Dependency"}, nil)
	orch.SubmitJob(ctx, WorkItem{ID: "JOB1", Summary: "Depends on DEP1", DependsOn: []string{"DEP1"}}, nil)

	// Before skip, JOB1 should not have dependencies met
	orch.mu.Lock()
	met, _, _, _, _ := orch.checkDependenciesMetLocked(WorkItem{DependsOn: []string{"DEP1"}})
	orch.mu.Unlock()
	assert.False(t, met)

	// Skip DEP1
	err := orch.SkipJob(ctx, "DEP1", nil)
	assert.NoError(t, err)

	// Now JOB1 dependencies should be met
	orch.mu.Lock()
	met, _, _, _, _ = orch.checkDependenciesMetLocked(WorkItem{DependsOn: []string{"DEP1"}})
	orch.mu.Unlock()
	assert.True(t, met)
}

func TestSkipJobDownstream(t *testing.T) {
	ctx := context.Background()
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	// Setup graph: A -> B -> C, D (independent)
	orch.pendingJobs["A"] = JobInfo{
		ID:       "A",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "A"},
	}
	orch.pendingJobs["B"] = JobInfo{
		ID:       "B",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "B", DependsOn: []string{"A"}},
	}
	orch.pendingJobs["C"] = JobInfo{
		ID:       "C",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "C", DependsOn: []string{"B"}},
	}
	orch.pendingJobs["D"] = JobInfo{
		ID:       "D",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "D"},
	}

	// Skip A and its downstream
	skippedIDs, err := orch.SkipJobDownstream(ctx, "A", nil)
	assert.NoError(t, err)

	// A, B, C should be skipped
	assert.ElementsMatch(t, []string{"A", "B", "C"}, skippedIDs)

	// Verify they are removed from pending queue
	_, okA := orch.pendingJobs["A"]
	_, okB := orch.pendingJobs["B"]
	_, okC := orch.pendingJobs["C"]
	assert.False(t, okA)
	assert.False(t, okB)
	assert.False(t, okC)

	// D should still be pending
	_, okD := orch.pendingJobs["D"]
	assert.True(t, okD)
}

func TestSkipJobDownstream_NotFound(t *testing.T) {
	ctx := context.Background()
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	skippedIDs, err := orch.SkipJobDownstream(ctx, "NON_EXISTENT", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, skippedIDs)
}

func TestOrchestrator_SkipJobsOlderThan(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)

	// Add an old job
	job1 := JobInfo{
		ID:        "JOB1",
		Status:    "Pending",
		StartTime: time.Now().Add(-2 * time.Hour), // 2 hours old
	}
	orch.pendingJobs["JOB1"] = job1

	// Add a new job
	job2 := JobInfo{
		ID:        "JOB2",
		Status:    "Pending",
		StartTime: time.Now().Add(-30 * time.Minute), // 30 minutes old
	}
	orch.pendingJobs["JOB2"] = job2

	// Skip jobs older than 1 hour
	count, err := orch.SkipJobsOlderThan(context.Background(), 1*time.Hour, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify job1 was skipped
	_, exists1 := orch.pendingJobs["JOB1"]
	assert.False(t, exists1)

	// It should be in history
	assert.Len(t, orch.completedJobs, 1)
	assert.Equal(t, "Skipped", orch.completedJobs[0].Status)
	assert.Equal(t, "JOB1", orch.completedJobs[0].ID)

	// Verify job2 was NOT skipped
	_, exists2 := orch.pendingJobs["JOB2"]
	assert.True(t, exists2)
}
