package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRetryJobDownstream(t *testing.T) {
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	spawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(poller, spawner, 1*time.Minute)

	// Create a chain of completed jobs: A -> B -> C
	jobA := JobInfo{
		ID:        "A",
		Status:    "Completed",
		WorkItem:  WorkItem{ID: "A"},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	jobB := JobInfo{
		ID:        "B",
		Status:    "Completed",
		WorkItem:  WorkItem{ID: "B", DependsOn: []string{"A"}},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	jobC := JobInfo{
		ID:        "C",
		Status:    "Failed", // C failed, but we are retrying from A
		WorkItem:  WorkItem{ID: "C", DependsOn: []string{"B"}},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	orch.completedJobs = []JobInfo{jobA, jobB, jobC}

	ctx := context.Background()

	// Test retrying A and downstream
	retriedIDs, err := orch.RetryJobDownstream(ctx, "A", nil)
	assert.NoError(t, err)

	// A, B, C should be retried (order not strictly guaranteed by BFS if there were branches, but it should contain all)
	assert.ElementsMatch(t, []string{"A", "B", "C"}, retriedIDs)

	// Wait a tiny bit for the goroutine to put A into activeJobs
	time.Sleep(50 * time.Millisecond)
}

func TestRetryJobDownstream_ActiveJob(t *testing.T) {
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, 1*time.Minute)

	// Job A is active
	orch.activeJobs["A"] = JobInfo{ID: "A", Status: "Running"}

	ctx := context.Background()

	retriedIDs, err := orch.RetryJobDownstream(ctx, "A", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is active")
	assert.Nil(t, retriedIDs)
}

func TestRetryJobDownstream_NotFound(t *testing.T) {
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, 1*time.Minute)

	ctx := context.Background()

	retriedIDs, err := orch.RetryJobDownstream(ctx, "NONEXISTENT", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in history")
	assert.Nil(t, retriedIDs)
}
