package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
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
