package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_CancelJobsByStatus(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	orch.activeJobs["JOB-1"] = JobInfo{ID: "JOB-1", Status: "Running"}
	orch.pendingJobs["JOB-2"] = JobInfo{ID: "JOB-2", Status: "Pending"}
	orch.pendingJobs["JOB-3"] = JobInfo{ID: "JOB-3", Status: "Pending Approval"}

	count, err := orch.CancelJobsByStatus(context.Background(), "Pending", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	orch.mu.RLock()
	_, exists := orch.pendingJobs["JOB-2"]
	orch.mu.RUnlock()
	assert.False(t, exists)
}

func TestOrchestrator_CancelJobsByMatch(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	orch.activeJobs["JOB-1"] = JobInfo{ID: "JOB-1", Summary: "login bug"}
	orch.pendingJobs["JOB-2"] = JobInfo{ID: "JOB-2", Summary: "logout bug"}

	count, err := orch.CancelJobsByMatch(context.Background(), "login", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestOrchestrator_CancelJobsByTag(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	orch.activeJobs["JOB-1"] = JobInfo{
		ID: "JOB-1",
		WorkItem: WorkItem{
			Tags: []string{"tag1", "target-tag"},
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID: "JOB-2",
		WorkItem: WorkItem{
			Tags: []string{"tag2"},
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID: "JOB-3",
		WorkItem: WorkItem{
			Tags: []string{"target-tag", "tag3"},
		},
	}

	count, err := orch.CancelJobsByTag(context.Background(), "TARGET-TAG", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}
