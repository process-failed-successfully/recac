package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_Spawn_PlanOnly(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:       "PLAN-ONLY-JOB",
		RepoURL:  "https://github.com/test/repo",
		PlanOnly: true, // Enable PlanOnly
	}

	ctx := context.Background()

	// Mock expectations
	// Expect RunContainerWithLabels with command containing --plan
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.MatchedBy(func(cmd []string) bool {
		// cmd is [/bin/sh -c ...]
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		return contains(cmdStr, "--plan")
	}), "", mock.Anything).Return("container-plan", nil)

	// Expect WaitContainer
	mockDocker.On("WaitContainer", ctx, "container-plan").Return(int(0), nil)

	// Expect SessionManager calls
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "PLAN-ONLY-JOB").Return(&runner.SessionState{}, nil)

	// Expect Git calls
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)
	mockDocker.AssertExpectations(t)
}
