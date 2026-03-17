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

func TestUpdateJobsAgentByTag(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:     "JOB-1",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:   "JOB-1",
			Tags: []string{"backend", "urgent"},
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID:     "JOB-2",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:   "JOB-2",
			Tags: []string{"frontend"},
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID:     "JOB-3",
		Status: "Pending",
		WorkItem: WorkItem{
			ID:   "JOB-3",
			Tags: []string{"Backend"}, // Case insensitive check
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}

	count, err := orch.UpdateJobsAgentByTag(ctx, "backend", "anthropic", "claude-3", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, "anthropic", orch.pendingJobs["JOB-1"].WorkItem.AgentProvider)
	assert.Equal(t, "claude-3", orch.pendingJobs["JOB-1"].WorkItem.AgentModel)
	assert.Equal(t, "anthropic", orch.pendingJobs["JOB-3"].WorkItem.AgentProvider)
	assert.Equal(t, "claude-3", orch.pendingJobs["JOB-3"].WorkItem.AgentModel)

	// JOB-2 should be unchanged
	assert.Equal(t, "openai", orch.pendingJobs["JOB-2"].WorkItem.AgentProvider)
	assert.Equal(t, "gpt-3.5", orch.pendingJobs["JOB-2"].WorkItem.AgentModel)
}

func TestUpdateJobsAgentByMatch(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(mockPoller, mockSpawner, 1*time.Minute)

	ctx := context.Background()

	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:      "JOB-1",
		Status:  "Pending",
		Summary: "Fix login issue",
		WorkItem: WorkItem{
			ID: "JOB-1",
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID:      "JOB-2",
		Status:  "Pending",
		Summary: "Update dashboard",
		WorkItem: WorkItem{
			ID: "JOB-2",
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID:      "JOB-3",
		Status:  "Pending",
		Summary: "Login page crash",
		WorkItem: WorkItem{
			ID: "JOB-3",
			AgentProvider: "openai",
			AgentModel: "gpt-3.5",
			RunAfter: time.Now().Add(1 * time.Hour),
		},
	}

	count, err := orch.UpdateJobsAgentByMatch(ctx, "login", "google", "gemini-1.5-pro", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	assert.Equal(t, "google", orch.pendingJobs["JOB-1"].WorkItem.AgentProvider)
	assert.Equal(t, "gemini-1.5-pro", orch.pendingJobs["JOB-1"].WorkItem.AgentModel)
	assert.Equal(t, "google", orch.pendingJobs["JOB-3"].WorkItem.AgentProvider)
	assert.Equal(t, "gemini-1.5-pro", orch.pendingJobs["JOB-3"].WorkItem.AgentModel)

	// JOB-2 should be unchanged
	assert.Equal(t, "openai", orch.pendingJobs["JOB-2"].WorkItem.AgentProvider)
	assert.Equal(t, "gpt-3.5", orch.pendingJobs["JOB-2"].WorkItem.AgentModel)

	// Test invalid regex
	_, err = orch.UpdateJobsAgentByMatch(ctx, "[invalid", "anthropic", "claude-3", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid match regex")
}
