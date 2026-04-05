package runner

import (
	"recac/internal/agent"
	"recac/internal/db"
	"testing"
	"time"
    "path/filepath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_CanAcquireImmediate(t *testing.T) {
	// Setup
    dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	dockerCli := &MockOrchestratorDocker{}
	mockAgent := &agent.MockAgent{}

	o := NewOrchestrator(store, dockerCli, "/workspace", "image", mockAgent, "test-proj", "provider", "model", 1, "")
	o.Graph = NewTaskGraph()

	// Should be able to acquire immediate when DB is empty and graph is empty
	assert.True(t, o.canAcquireImmediate([]string{"path/to/file"}))

	// Acquire lock in DB
	acquired, err := store.AcquireLock("test-proj", "path/to/file", "task-1", 1*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)

	// Should NOT be able to acquire immediate when DB has lock
	assert.False(t, o.canAcquireImmediate([]string{"path/to/file"}))

	// Release lock in DB
	err = store.ReleaseLock("test-proj", "path/to/file", "task-1")
	require.NoError(t, err)

	// Check with local graph lock
	o.Graph.AddNode("task-2", "Task 2", []string{})
	o.Graph.Nodes["task-2"].ExclusiveWritePaths = []string{"path/to/other"}
	o.Graph.Nodes["task-2"].Status = TaskInProgress

	assert.False(t, o.canAcquireImmediate([]string{"path/to/other"}))
	assert.True(t, o.canAcquireImmediate([]string{"path/to/different"}))
}
