package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"recac/internal/runner"

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
		ID:       "PLAN-ONLY-TICKET",
		RepoURL:  "https://github.com/test/repo",
		PlanOnly: true,
	}

	ctx := context.Background()

	// Mock expectations
	// Expect RunContainerWithLabels. We verify that the command (arg 5) contains "--plan"
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.MatchedBy(func(cmd []string) bool {
		// cmd is []string{"/bin/sh", "-c", "..."}
		if len(cmd) < 3 {
			return false
		}
		shellCmd := cmd[2]
		return strings.Contains(shellCmd, "--plan")
	}), mock.Anything, mock.Anything).Return("container-plan", nil)

	// Expect WaitContainer
	mockDocker.On("WaitContainer", ctx, "container-plan").Return(int(0), nil)

	// Mock SessionManager
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "PLAN-ONLY-TICKET").Return(&runner.SessionState{}, nil)

	// Mock Git
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("endsha", nil).Maybe()

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)
	mockDocker.AssertExpectations(t)
}
