package orchestrator

import (
	"context"
	"io"
	"log/slog"
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
		ID:       "PLAN-TEST-1",
		RepoURL:  "https://github.com/test/repo",
		PlanOnly: true,
	}

	ctx := context.Background()

	// Capture command from RunContainerWithLabels
	var capturedCmd []string
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(cmd []string) bool {
		capturedCmd = cmd
		return true
	}), "", mock.Anything).Return("container123", nil)

	mockDocker.On("WaitContainer", ctx, "container123").Return(int(0), nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "PLAN-TEST-1").Return(&runner.SessionState{}, nil)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	// Verify command contains --plan
	// cmd is ["/bin/sh", "-c", "string"]
	assert.Len(t, capturedCmd, 3)
	cmdStr := capturedCmd[2]
	assert.Contains(t, cmdStr, "--plan")
}
