package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateJobAgent(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	// Add a pending job
	jobID := "JOB-AGENT-1"
	orch.pendingJobs[jobID] = JobInfo{
		ID:     jobID,
		Status: "Pending",
		WorkItem: WorkItem{
			ID:            jobID,
			AgentProvider: "openai",
			AgentModel:    "gpt-4",
			RunAfter:      time.Now().Add(1 * time.Hour), // Prevent evaluatePendingJobs from eating it
		},
	}

	// 1. Successful update
	err := orch.UpdateJobAgent(ctx, jobID, "anthropic", "claude-3", nil)
	assert.NoError(t, err)

	updatedJob := orch.pendingJobs[jobID]
	assert.Equal(t, "anthropic", updatedJob.WorkItem.AgentProvider)
	assert.Equal(t, "claude-3", updatedJob.WorkItem.AgentModel)

	// 2. Partial update (only provider)
	err = orch.UpdateJobAgent(ctx, jobID, "google", "", nil)
	assert.NoError(t, err)
	updatedJob = orch.pendingJobs[jobID]
	assert.Equal(t, "google", updatedJob.WorkItem.AgentProvider)
	assert.Equal(t, "claude-3", updatedJob.WorkItem.AgentModel)

	// 3. Partial update (only model)
	err = orch.UpdateJobAgent(ctx, jobID, "", "gemini-1.5-pro", nil)
	assert.NoError(t, err)
	updatedJob = orch.pendingJobs[jobID]
	assert.Equal(t, "google", updatedJob.WorkItem.AgentProvider)
	assert.Equal(t, "gemini-1.5-pro", updatedJob.WorkItem.AgentModel)

	// 4. Update active job (should fail)
	activeJobID := "JOB-ACTIVE-1"
	orch.activeJobs[activeJobID] = JobInfo{
		ID:     activeJobID,
		Status: "Running",
	}
	err = orch.UpdateJobAgent(ctx, activeJobID, "openai", "gpt-4", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// 5. Update completed job (should fail)
	completedJobID := "JOB-COMPLETED-1"
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     completedJobID,
		Status: "Completed",
	})
	err = orch.UpdateJobAgent(ctx, completedJobID, "openai", "gpt-4", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")

	// 6. Update non-existent job (should fail)
	err = orch.UpdateJobAgent(ctx, "NON-EXISTENT", "openai", "gpt-4", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
