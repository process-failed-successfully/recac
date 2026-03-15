package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_UpdateDependencies(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 1*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Submit a pending job
	job1 := WorkItem{ID: "JOB-1"}
	orch.SubmitJob(ctx, job1, nil)

	// It should be active because it has no dependencies
	jobInfo, err := orch.GetJob("JOB-1")
	assert.NoError(t, err)
	assert.Equal(t, "Spawning", jobInfo.Status) // Job-1 starts immediately

	// Submit a job with a dependency that doesn't exist yet
	job2 := WorkItem{ID: "JOB-2", DependsOn: []string{"JOB-999"}}
	orch.SubmitJob(ctx, job2, nil)

	jobInfo, err = orch.GetJob("JOB-2")
	assert.NoError(t, err)
	assert.Equal(t, "Pending", jobInfo.Status)

	// Update dependencies
	err = orch.UpdateJobDependencies(ctx, "JOB-2", []string{"JOB-1"}, nil)
	assert.NoError(t, err)

	jobInfo, err = orch.GetJob("JOB-2")
	assert.NoError(t, err)
	assert.Equal(t, []string{"JOB-1"}, jobInfo.WorkItem.DependsOn)

	// Verify cycle detection
	job3 := WorkItem{ID: "JOB-3", DependsOn: []string{"JOB-2"}}
	orch.SubmitJob(ctx, job3, nil)

	err = orch.UpdateJobDependencies(ctx, "JOB-2", []string{"JOB-3"}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")

	// Dependencies should be unchanged
	jobInfo, err = orch.GetJob("JOB-2")
	assert.NoError(t, err)
	assert.Equal(t, []string{"JOB-1"}, jobInfo.WorkItem.DependsOn)
}

func TestOrchestrator_UpdateDependenciesBulk(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 1*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	job1 := WorkItem{ID: "JOB-1", Tags: []string{"tag-a"}, Summary: "match me 1", DependsOn: []string{"INITIAL"}}
	job2 := WorkItem{ID: "JOB-2", Tags: []string{"tag-b"}, Summary: "match me 2", DependsOn: []string{"INITIAL"}}
	job3 := WorkItem{ID: "JOB-3", Tags: []string{"tag-a"}, Summary: "no match", DependsOn: []string{"INITIAL"}}

	orch.SubmitJob(ctx, job1, nil)
	orch.SubmitJob(ctx, job2, nil)
	orch.SubmitJob(ctx, job3, nil)

	// Test UpdateJobsDependenciesByTag
	count, err := orch.UpdateJobsDependenciesByTag(ctx, "tag-a", []string{"DEP-TAG"}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	jobInfo, _ := orch.GetJob("JOB-1")
	assert.Equal(t, []string{"DEP-TAG"}, jobInfo.WorkItem.DependsOn)
	jobInfo, _ = orch.GetJob("JOB-3")
	assert.Equal(t, []string{"DEP-TAG"}, jobInfo.WorkItem.DependsOn)
	jobInfo, _ = orch.GetJob("JOB-2")
	assert.Equal(t, []string{"INITIAL"}, jobInfo.WorkItem.DependsOn) // Unchanged

	// Reset
	orch.UpdateJobDependencies(ctx, "JOB-1", []string{"INITIAL"}, nil)
	orch.UpdateJobDependencies(ctx, "JOB-3", []string{"INITIAL"}, nil)

	// Test UpdateJobsDependenciesByMatch
	count, err = orch.UpdateJobsDependenciesByMatch(ctx, "match me", []string{"DEP-MATCH"}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	jobInfo, _ = orch.GetJob("JOB-1")
	assert.Equal(t, []string{"DEP-MATCH"}, jobInfo.WorkItem.DependsOn)
	jobInfo, _ = orch.GetJob("JOB-2")
	assert.Equal(t, []string{"DEP-MATCH"}, jobInfo.WorkItem.DependsOn)
	jobInfo, _ = orch.GetJob("JOB-3")
	assert.Equal(t, []string{"INITIAL"}, jobInfo.WorkItem.DependsOn) // Unchanged

	// Test invalid match
	_, err = orch.UpdateJobsDependenciesByMatch(ctx, "[", []string{"DEP-MATCH"}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid match regex")
}
