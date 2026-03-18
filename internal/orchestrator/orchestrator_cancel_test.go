package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestCancelJobDownstream(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockPoller := new(MockPoller)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)

	// Add some active and pending jobs to simulate a dependency graph
	orch.mu.Lock()

	// A -> B -> C
	// A -> D

	orch.activeJobs["A"] = JobInfo{
		ID:     "A",
		Status: "Active",
		WorkItem: WorkItem{
			ID: "A",
		},
	}

	orch.pendingJobs["B"] = JobInfo{
		ID:     "B",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "B",
			DependsOn: []string{"A"},
		},
	}

	orch.pendingJobs["C"] = JobInfo{
		ID:     "C",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "C",
			DependsOn: []string{"B"},
		},
	}

	orch.pendingJobs["D"] = JobInfo{
		ID:     "D",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:        "D",
			DependsOn: []string{"A"},
		},
	}
	orch.mu.Unlock()

	ctx := context.Background()

	// Cancel A and its downstream
	canceledIDs, err := orch.CancelJobDownstream(ctx, "A", nil)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"A", "B", "C", "D"}, canceledIDs)

	// Verify all were removed from active/pending (or handled by CancelJob)
	// CancelJob removes from pending queue directly, but for activeJobs it relies on the spawner
	// which is mocked here to just return nil, so we only check pending.
	orch.mu.RLock()
	_, bExists := orch.pendingJobs["B"]
	_, cExists := orch.pendingJobs["C"]
	_, dExists := orch.pendingJobs["D"]
	orch.mu.RUnlock()

	assert.False(t, bExists)
	assert.False(t, cExists)
	assert.False(t, dExists)
}

func TestCancelJobDownstream_NotFound(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockPoller := new(MockPoller)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	canceledIDs, err := orch.CancelJobDownstream(ctx, "NON_EXISTENT", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in active or pending queue")
	assert.Nil(t, canceledIDs)
}
