package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_Persistence_Integration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-orch-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "history.db")

	// 1. Start Orchestrator 1 and run a job
	{
		poller := newMockPoller(nil)
		spawner := &mockSpawner{}
		orch := New(poller, spawner, 50*time.Millisecond)

		p := NewSQLitePersistence(dbPath)
		require.NoError(t, p.Init())
		// We don't defer Close() here because we want to close it before opening in next block
		// Although SQLite handles multiple connections, best to be clean.
		orch.SetPersistence(p)

		ctx := context.Background()
		item := WorkItem{ID: "PERSIST-1", Summary: "To be persisted"}

		// Use SubmitJob to trigger processing
		err := orch.SubmitJob(ctx, item, silentLogger)
		require.NoError(t, err)

		// Wait for job to complete
		require.Eventually(t, func() bool {
			// Check DB directly
			job, err := p.GetJob("PERSIST-1")
			return err == nil && job.Status == "Completed"
		}, 1*time.Second, 10*time.Millisecond)

		p.Close()
	}

	// 2. Start Orchestrator 2 and load history
	{
		poller := newMockPoller(nil)
		spawner := &mockSpawner{}
		orch := New(poller, spawner, 50*time.Millisecond)

		p := NewSQLitePersistence(dbPath)
		require.NoError(t, p.Init())
		defer p.Close()
		orch.SetPersistence(p)

		// Verify empty history initially
		assert.Empty(t, orch.GetCompletedJobs())

		// Load History
		err := orch.LoadHistory(silentLogger)
		require.NoError(t, err)

		// Verify history loaded
		completed := orch.GetCompletedJobs()
		require.Len(t, completed, 1)
		assert.Equal(t, "PERSIST-1", completed[0].ID)
		assert.Equal(t, "To be persisted", completed[0].Summary)
		assert.Equal(t, "Completed", completed[0].Status)

		// Verify GetJob finds it
		job, err := orch.GetJob("PERSIST-1")
		require.NoError(t, err)
		assert.Equal(t, "PERSIST-1", job.ID)
	}
}
