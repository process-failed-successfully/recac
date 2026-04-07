package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockLockDBStore embeds MockDBStore and overrides locking for this test
type mockLockDBStore struct {
	MockDBStore
	acquireLockFunc func(project, path, owner string, expires time.Duration) (bool, error)
	releaseLockFunc func(project, path, owner string) error
}

func (m *mockLockDBStore) AcquireLock(project, path, owner string, expires time.Duration) (bool, error) {
	if m.acquireLockFunc != nil {
		return m.acquireLockFunc(project, path, owner, expires)
	}
	return true, nil
}

func (m *mockLockDBStore) ReleaseLock(project, path, owner string) error {
	if m.releaseLockFunc != nil {
		return m.releaseLockFunc(project, path, owner)
	}
	return nil
}

func TestOrchestrator_ExecuteTask_Coverage(t *testing.T) {
	dbStore := &mockLockDBStore{}
	mockDocker := new(MockOrchestratorDocker)
	mockAgent := new(MockOrchestratorAgent)

	o := NewOrchestrator(dbStore, mockDocker, "/test/workspace", "test-image", mockAgent, "test-project", "openai", "gpt-4", 1, "")
	o.Graph = NewTaskGraph()

	ctx := context.Background()

	t.Run("LockAcquisitionFailure", func(t *testing.T) {
		taskID := "task-lock-fail"
		node := &TaskNode{
			ID:                  taskID,
			Status:              TaskPending,
			ExclusiveWritePaths: []string{"/shared/path"},
		}
		o.Graph.Nodes[taskID] = node

		// Mock AcquireLock to fail
		dbStore.acquireLockFunc = func(project, path, owner string, expires time.Duration) (bool, error) {
			return false, nil
		}

		err := o.ExecuteTask(ctx, taskID, node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lock acquisition failed")

		// Status should be pending with error
		status, _ := o.Graph.GetTaskStatus(taskID)
		assert.Equal(t, TaskPending, status)
		// MarkTaskStatus stores error natively, but GetTaskStatus returns wrapped error if not found,
		// so errStr depends on implementation detail. Just checking status is fine.
	})

	t.Run("StatusChangedBeforeStart", func(t *testing.T) {
		taskID := "task-status-changed"
		node := &TaskNode{
			ID:      taskID,
			Status:  TaskDone, // Already completed
		}
		o.Graph.Nodes[taskID] = node

		dbStore.acquireLockFunc = func(project, path, owner string, expires time.Duration) (bool, error) {
			return true, nil
		}
		dbStore.releaseLockFunc = func(project, path, owner string) error {
			return nil
		}

		err := o.ExecuteTask(ctx, taskID, node)
		assert.NoError(t, err) // Returns nil, skips execution
	})
}
