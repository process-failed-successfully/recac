package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"strings"
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
		ID:       "PLAN-TICKET",
		RepoURL:  "https://github.com/test/repo",
		PlanOnly: true,
	}

	ctx := context.Background()

	// Mock expectations
	// Verify RunContainerWithLabels is called with a command containing --plan
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(cmd []string) bool {
		// cmd is ["/bin/sh", "-c", "script..."]
		// script should contain --plan
		if len(cmd) < 3 {
			return false
		}
		script := cmd[2]
		return strings.Contains(script, "--plan")
	}), mock.Anything, mock.Anything).Return("container-plan", nil)

	mockDocker.On("WaitContainer", ctx, "container-plan").Return(int(0), nil)

	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "PLAN-TICKET").Return(&runner.SessionState{}, nil)

	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)
	mockDocker.AssertExpectations(t)
}
