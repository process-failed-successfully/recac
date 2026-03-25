package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"fmt"
	"github.com/stretchr/testify/require"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSetJobOutput_ActiveJob(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch.activeJobs["job-1"] = JobInfo{
		ID:     "job-1",
		Status: "Active",
	}

	outputs := map[string]string{"foo": "bar"}
	err := orch.SetJobOutput("job-1", outputs, logger)
	assert.NoError(t, err)

	job, _ := orch.GetJob("job-1")
	assert.Equal(t, outputs, job.Outputs)
}

func TestSetJobOutput_PendingJob(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch.pendingJobs["job-1"] = JobInfo{
		ID:     "job-1",
		Status: "Pending",
	}

	outputs := map[string]string{"foo": "bar"}
	err := orch.SetJobOutput("job-1", outputs, logger)
	assert.NoError(t, err)

	job, _ := orch.GetJob("job-1")
	assert.Equal(t, outputs, job.Outputs)
}

func TestSetJobOutput_NonExistentJob(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	outputs := map[string]string{"foo": "bar"}
	err := orch.SetJobOutput("non-existent-job", outputs, logger)
	assert.Error(t, err)
}

func TestSetJobOutput_CompletedJob(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "job-1",
		Status: "Completed",
	})

	outputs := map[string]string{"foo": "bar"}
	err := orch.SetJobOutput("job-1", outputs, logger)
	assert.NoError(t, err)

	job, _ := orch.GetJob("job-1")
	assert.Equal(t, outputs, job.Outputs)
}

func TestSetJobOutput_DependencyChaining(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Minute)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Pre-populate completed job A with an output
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "JOB-A",
		Status: "Completed",
		Outputs: map[string]string{
			"url": "https://example.com",
		},
	})

	// Job B depends on Job A
	itemB := WorkItem{
		ID:        "JOB-B",
		DependsOn: []string{"JOB-A"},
		EnvVars: map[string]string{
			"EXISTING": "VALUE",
		},
	}

	// We need to wait for Spawn to be called since processWorkItem internal calls it in a goroutine
	done := make(chan struct{})
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		defer close(done)
		// Verify env vars
		if item.EnvVars["EXISTING"] != "VALUE" {
			return false
		}
		if item.EnvVars["DEP_JOB_A_URL"] != "https://example.com" {
			return false
		}
		return true
	})).Return(nil)

	err := orch.SubmitJob(context.Background(), itemB, logger)
	assert.NoError(t, err)

	// Wait for goroutine
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for Spawner.Spawn")
	}

	// Spawner should be called
	mockSpawner.AssertExpectations(t)
}

type mockOutputPersistence struct {
	savedJobs map[string]JobInfo
	saveErr   error
	getErr    error
}

func (m *mockOutputPersistence) Init() error                               { return nil }
func (m *mockOutputPersistence) SaveJob(job JobInfo) error                 {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.savedJobs == nil {
		m.savedJobs = make(map[string]JobInfo)
	}
	m.savedJobs[job.ID] = job
	return nil
}
func (m *mockOutputPersistence) GetJob(id string) (*JobInfo, error)        {
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
func (m *mockOutputPersistence) GetJobs(limit int) ([]JobInfo, error)      { return nil, nil }
func (m *mockOutputPersistence) Close() error                              { return nil }
func (m *mockOutputPersistence) ClearHistory() (int, error)                { return 0, nil }
func (m *mockOutputPersistence) PurgeJob(id string) error                  { return nil }

func TestSetJobOutput_Persistence(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	jobID := "PERSISTED-JOB"

	t.Run("Success", func(t *testing.T) {
		mockDB := &mockOutputPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
		}
		orch.SetPersistence(mockDB)

		outputs := map[string]string{"result": "success"}
		err := orch.SetJobOutput(jobID, outputs, logger)
		require.NoError(t, err)

		job, err := mockDB.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, "success", job.Outputs["result"])
	})

	t.Run("SaveJobError", func(t *testing.T) {
		mockDB := &mockOutputPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		outputs := map[string]string{"result": "success"}
		err := orch.SetJobOutput(jobID, outputs, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to persist")
	})

	t.Run("MemoryAndSaveJobError", func(t *testing.T) {
		mockDB := &mockOutputPersistence{
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "MEM-JOB"})

		outputs := map[string]string{"result": "success"}
		err := orch.SetJobOutput("MEM-JOB", outputs, logger)
		require.NoError(t, err)

		orch.completedJobs = orch.completedJobs[:len(orch.completedJobs)-1]
	})
}
