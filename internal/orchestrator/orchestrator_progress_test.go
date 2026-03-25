package orchestrator

import (
	"io"
	"log/slog"
	"testing"
	"fmt"
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

type mockProgressPersistence struct {
	savedJobs map[string]JobInfo
	saveErr   error
	getErr    error
}

func (m *mockProgressPersistence) Init() error                               { return nil }
func (m *mockProgressPersistence) SaveJob(job JobInfo) error                 {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.savedJobs == nil {
		m.savedJobs = make(map[string]JobInfo)
	}
	m.savedJobs[job.ID] = job
	return nil
}
func (m *mockProgressPersistence) GetJob(id string) (*JobInfo, error)        {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.savedJobs != nil {
		if job, ok := m.savedJobs[id]; ok {
			return &job, nil
		}
	}
	return nil, fmt.Errorf("job %s not found", id)
}
func (m *mockProgressPersistence) GetJobs(limit int) ([]JobInfo, error)      { return nil, nil }
func (m *mockProgressPersistence) Close() error                              { return nil }
func (m *mockProgressPersistence) ClearHistory() (int, error)                { return 0, nil }
func (m *mockProgressPersistence) PurgeJob(id string) error                  { return nil }

func TestUpdateJobProgress_Persistence(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	jobID := "PERSISTED-JOB"

	t.Run("Success", func(t *testing.T) {
		mockDB := &mockProgressPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
		}
		orch.SetPersistence(mockDB)

		progressVal := 50
		msg := "halfway"
		err := orch.UpdateJobProgress(jobID, &progressVal, &msg, logger)
		require.NoError(t, err)

		job, err := mockDB.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, 50, *job.Progress)
		assert.Equal(t, "halfway", *job.StatusMessage)
	})

	t.Run("SaveJobError", func(t *testing.T) {
		mockDB := &mockProgressPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		progressVal := 50
		msg := "halfway"
		err := orch.UpdateJobProgress(jobID, &progressVal, &msg, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to persist")
	})

	t.Run("MemoryAndSaveJobError", func(t *testing.T) {
		mockDB := &mockProgressPersistence{
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "MEM-JOB"})

		progressVal := 50
		msg := "halfway"
		err := orch.UpdateJobProgress("MEM-JOB", &progressVal, &msg, logger)
		require.NoError(t, err)

		orch.completedJobs = orch.completedJobs[:len(orch.completedJobs)-1]
	})
}
