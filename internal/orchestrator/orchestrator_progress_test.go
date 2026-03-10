package orchestrator

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_UpdateJobProgress(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	jobIDActive := "JOB-ACTIVE"
	orch.activeJobs[jobIDActive] = JobInfo{
		ID:        jobIDActive,
		Status:    "Running",
		StartTime: time.Now(),
	}

	jobIDPending := "JOB-PENDING"
	orch.pendingJobs[jobIDPending] = JobInfo{
		ID:        jobIDPending,
		Status:    "Pending",
		StartTime: time.Now(),
	}

	jobIDHistory := "JOB-HISTORY"
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:        jobIDHistory,
		Status:    "Completed",
		StartTime: time.Now(),
	})

	progressVal := 75
	msg := "Processing data"

	t.Run("Update Active Job", func(t *testing.T) {
		err := orch.UpdateJobProgress(jobIDActive, &progressVal, &msg, logger)
		require.NoError(t, err)

		job, _ := orch.GetJob(jobIDActive)
		require.NotNil(t, job.Progress)
		assert.Equal(t, 75, *job.Progress)
		require.NotNil(t, job.StatusMessage)
		assert.Equal(t, "Processing data", *job.StatusMessage)
	})

	t.Run("Update Pending Job", func(t *testing.T) {
		progressVal2 := 10
		msg2 := "Waiting in queue"
		err := orch.UpdateJobProgress(jobIDPending, &progressVal2, &msg2, logger)
		require.NoError(t, err)

		job, _ := orch.GetJob(jobIDPending)
		require.NotNil(t, job.Progress)
		assert.Equal(t, 10, *job.Progress)
		require.NotNil(t, job.StatusMessage)
		assert.Equal(t, "Waiting in queue", *job.StatusMessage)
	})

	t.Run("Update History Job", func(t *testing.T) {
		progressVal3 := 100
		msg3 := "Done"
		err := orch.UpdateJobProgress(jobIDHistory, &progressVal3, &msg3, logger)
		require.NoError(t, err)

		job, _ := orch.GetJob(jobIDHistory)
		require.NotNil(t, job.Progress)
		assert.Equal(t, 100, *job.Progress)
		require.NotNil(t, job.StatusMessage)
		assert.Equal(t, "Done", *job.StatusMessage)
	})

	t.Run("Update Non-Existent Job", func(t *testing.T) {
		err := orch.UpdateJobProgress("JOB-UNKNOWN", &progressVal, &msg, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
