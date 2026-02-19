package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLitePersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	p := NewSQLitePersistence(dbPath)
	defer p.Close()

	t.Run("Init", func(t *testing.T) {
		err := p.Init()
		require.NoError(t, err)
		assert.FileExists(t, dbPath)
	})

	job1 := JobInfo{
		ID:        "JOB-1",
		Summary:   "Test Job 1",
		StartTime: time.Now().Add(-1 * time.Hour),
		Status:    "Completed",
		WorkItem: WorkItem{
			ID: "JOB-1", Description: "Desc 1",
		},
	}

	job2 := JobInfo{
		ID:        "JOB-2",
		Summary:   "Test Job 2",
		StartTime: time.Now(),
		Status:    "Running",
		WorkItem: WorkItem{
			ID: "JOB-2", Description: "Desc 2",
		},
	}

	t.Run("SaveJob", func(t *testing.T) {
		err := p.SaveJob(job1)
		require.NoError(t, err)

		err = p.SaveJob(job2)
		require.NoError(t, err)
	})

	t.Run("GetJob", func(t *testing.T) {
		got, err := p.GetJob("JOB-1")
		require.NoError(t, err)
		assert.Equal(t, job1.ID, got.ID)
		assert.Equal(t, job1.Summary, got.Summary)
		// Compare time carefully (JSON marshaling might lose precision/timezone)
		assert.WithinDuration(t, job1.StartTime, got.StartTime, time.Second)

		got2, err := p.GetJob("JOB-2")
		require.NoError(t, err)
		assert.Equal(t, job2.ID, got2.ID)

		_, err = p.GetJob("NON-EXISTENT")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetJobs", func(t *testing.T) {
		jobs, err := p.GetJobs(10)
		require.NoError(t, err)
		require.Len(t, jobs, 2)

		// Order should be DESC by CreatedAt (StartTime)
		assert.Equal(t, "JOB-2", jobs[0].ID)
		assert.Equal(t, "JOB-1", jobs[1].ID)

		// Limit
		jobs, err = p.GetJobs(1)
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, "JOB-2", jobs[0].ID)
	})

	t.Run("UpdateJob", func(t *testing.T) {
		job1Updated := job1
		job1Updated.Status = "Failed"
		err := p.SaveJob(job1Updated)
		require.NoError(t, err)

		got, err := p.GetJob("JOB-1")
		require.NoError(t, err)
		assert.Equal(t, "Failed", got.Status)
	})
}
