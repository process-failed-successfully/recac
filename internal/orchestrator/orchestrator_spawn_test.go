package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPersistenceError struct{}

func (m *mockPersistenceError) Init() error               { return nil }
func (m *mockPersistenceError) Close() error              { return nil }
func (m *mockPersistenceError) SaveJob(job JobInfo) error { return errors.New("save failed") }
func (m *mockPersistenceError) GetJob(id string) (*JobInfo, error) {
	return nil, errors.New("get failed")
}
func (m *mockPersistenceError) GetJobs(limit int) ([]JobInfo, error) {
	return nil, errors.New("list failed")
}
func (m *mockPersistenceError) ClearHistory() (int, error) {
	return 0, errors.New("clear failed")
}
func (m *mockPersistenceError) PurgeJob(id string) error {
	return errors.New("purge failed")
}

func TestOrchestrator_SpawnWorker_Failure(t *testing.T) {
	// 1. Setup Poller with 1 item
	poller := newMockPoller([]WorkItem{{ID: "FAIL-1"}})

	// 2. Setup Spawner that fails
	spawner := &mockSpawner{spawnErr: errors.New("docker connection error")}

	orch := New(poller, spawner, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// 3. Run
	err := orch.Run(ctx, silentLogger)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// 4. Verify History
	// Should be in history as Failed
	completed := orch.GetCompletedJobs()
	require.Len(t, completed, 1)
	assert.Equal(t, "FAIL-1", completed[0].ID)
	assert.Equal(t, "Failed", completed[0].Status)
	assert.Contains(t, completed[0].Error, "docker connection error")

	// 5. Verify Status Update in Poller
	poller.updateStatusMu.Lock()
	status, exists := poller.updateStatus["FAIL-1"]
	poller.updateStatusMu.Unlock()
	assert.True(t, exists)
	assert.Equal(t, "Failed", status)
}

func TestOrchestrator_LoadHistory_Failure(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)
	orch.SetPersistence(&mockPersistenceError{})

	err := orch.LoadHistory(silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}

func TestOrchestrator_AddToHistory_PersistenceFailure(t *testing.T) {
	// Verify that addToHistory logs error but doesn't crash if persistence fails.

	orch := New(newMockPoller([]WorkItem{{ID: "TEST-1"}}), &mockSpawner{}, 50*time.Millisecond)
	orch.SetPersistence(&mockPersistenceError{})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, silentLogger)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// History in memory should still be updated
	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
}

func TestOrchestrator_GetJob_PersistenceFailure(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)
	orch.SetPersistence(&mockPersistenceError{})

	_, err := orch.GetJob("MISSING")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
