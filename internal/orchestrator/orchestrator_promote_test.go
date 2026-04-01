package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromoteJob(t *testing.T) {
	orch := &Orchestrator{
		pendingJobs: map[string]JobInfo{
			"job1": {WorkItem: WorkItem{ID: "job1", Priority: 5}},
			"job2": {WorkItem: WorkItem{ID: "job2", Priority: 10}},
			"job3": {WorkItem: WorkItem{ID: "job3", Priority: 2}},
		},
	}

	newPriority, err := orch.PromoteJob(context.Background(), "job3", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 11, newPriority)
	assert.Equal(t, 11, orch.pendingJobs["job3"].WorkItem.Priority)

	// Promote job2, which is now no longer highest, job3 is 11
	newPriority, err = orch.PromoteJob(context.Background(), "job2", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 12, newPriority)
	assert.Equal(t, 12, orch.pendingJobs["job2"].WorkItem.Priority)

	// Promote non-existent job
	_, err = orch.PromoteJob(context.Background(), "job_missing", silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in pending queue")
}

func TestPromoteJob_EmptyOtherJobs(t *testing.T) {
	orch := &Orchestrator{
		pendingJobs: map[string]JobInfo{
			"job1": {WorkItem: WorkItem{ID: "job1", Priority: 0}},
		},
	}

	newPriority, err := orch.PromoteJob(context.Background(), "job1", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 1, newPriority)
	assert.Equal(t, 1, orch.pendingJobs["job1"].WorkItem.Priority)
}
