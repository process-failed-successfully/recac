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
