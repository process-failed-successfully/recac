package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCircularDependencyDetection(t *testing.T) {
	tests := []struct {
		name          string
		existingJobs  []WorkItem // These will be added to pendingJobs manually to setup state
		newJobs       []WorkItem // These will be submitted
		expectedError string
	}{
		{
			name:         "No dependencies",
			existingJobs: []WorkItem{},
			newJobs: []WorkItem{
				{ID: "A"},
			},
			expectedError: "",
		},
		{
			name: "Linear dependency",
			existingJobs: []WorkItem{
				{ID: "B"},
			},
			newJobs: []WorkItem{
				{ID: "A", DependsOn: []string{"B"}},
			},
			expectedError: "",
		},
		{
			name:         "Self dependency",
			existingJobs: []WorkItem{},
			newJobs: []WorkItem{
				{ID: "A", DependsOn: []string{"A"}},
			},
			expectedError: "circular dependency detected: A -> A",
		},
		{
			name: "Direct circular dependency A->B->A",
			existingJobs: []WorkItem{
				{ID: "B", DependsOn: []string{"A"}},
			},
			newJobs: []WorkItem{
				{ID: "A", DependsOn: []string{"B"}},
			},
			expectedError: "circular dependency detected: A -> B -> A",
		},
		{
			name: "Indirect circular dependency A->B->C->A",
			existingJobs: []WorkItem{
				{ID: "B", DependsOn: []string{"C"}},
				{ID: "C", DependsOn: []string{"A"}},
			},
			newJobs: []WorkItem{
				{ID: "A", DependsOn: []string{"B"}},
			},
			expectedError: "circular dependency detected: A -> B -> C -> A",
		},
		{
			name: "Cycle independent of new job",
			// A->B->A is a cycle, but we are submitting C->D.
			// Wait, the orchestrator prevents cycles from forming, so A->B->A shouldn't be in pendingJobs
			// unless forced. If it is forced, submitting C->D shouldn't fail because it doesn't touch the cycle.
			existingJobs: []WorkItem{
				{ID: "A", DependsOn: []string{"B"}},
				{ID: "B", DependsOn: []string{"A"}},
				{ID: "D"},
			},
			newJobs: []WorkItem{
				{ID: "C", DependsOn: []string{"D"}},
			},
			expectedError: "",
		},
		{
			name: "New job joins an existing cycle",
			existingJobs: []WorkItem{
				{ID: "B", DependsOn: []string{"A"}},
			},
			newJobs: []WorkItem{
				// A->B exists, now C->B. This is acyclic.
				{ID: "C", DependsOn: []string{"B"}},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poller := &MockPoller{}
			spawner := &MockSpawner{}
			spawner.On("Spawn", context.Background(), mock.Anything).Return(nil).Maybe()
			orch := New(poller, spawner, time.Minute)

			// Manually populate pendingJobs to simulate existing state
			orch.mu.Lock()
			for _, item := range tt.existingJobs {
				orch.pendingJobs[item.ID] = JobInfo{
					ID:       item.ID,
					Status:   "Pending",
					WorkItem: item,
				}
			}
			orch.mu.Unlock()

			var lastErr error
			for _, item := range tt.newJobs {
				err := orch.SubmitJob(context.Background(), item, nil)
				if err != nil {
					lastErr = err
					break
				}
			}

			if tt.expectedError != "" {
				assert.Error(t, lastErr)
				assert.Contains(t, lastErr.Error(), tt.expectedError)
			} else {
				assert.NoError(t, lastErr)
			}
		})
	}
}

func TestCircularDependencyAtomicBatch(t *testing.T) {
	// Let's test how it handles a sequence of submissions that would form a cycle
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	spawner.On("Spawn", context.Background(), mock.Anything).Return(nil).Maybe()
	orch := New(poller, spawner, time.Minute)

	// Submit A -> B
	err := orch.SubmitJob(context.Background(), WorkItem{ID: "A", DependsOn: []string{"B"}}, nil)
	assert.NoError(t, err)

	// Submit B -> C
	err = orch.SubmitJob(context.Background(), WorkItem{ID: "B", DependsOn: []string{"C"}}, nil)
	assert.NoError(t, err)

	// Submit C -> A (this should fail)
	err = orch.SubmitJob(context.Background(), WorkItem{ID: "C", DependsOn: []string{"A"}}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected: C -> A -> B -> C")

	// A and B should still be pending
	orch.mu.RLock()
	assert.Contains(t, orch.pendingJobs, "A")
	assert.Contains(t, orch.pendingJobs, "B")
	assert.NotContains(t, orch.pendingJobs, "C")
	orch.mu.RUnlock()
}
