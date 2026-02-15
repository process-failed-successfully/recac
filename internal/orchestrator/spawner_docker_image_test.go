package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Mock expectations
	mockDocker.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, nil)

	// Relax matchers to avoid test flakiness due to argument mismatch (e.g., nil vs empty slice)
	// Arguments: ctx, image, workspace, binds, env, labels, user
	mockDocker.On("RunContainer", mock.Anything, imageName, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container123", nil)

	mockSM.On("SaveSession", mock.Anything).Return(nil)
	// LoadSession is called in the goroutine after Exec
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	execCalled := make(chan string, 1)

	// Capture command. Match any args to ensure the call is captured even if container ID or context slightly differs.
	// Arguments: ctx, containerID, cmd
	mockDocker.On("Exec", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		if len(cmd) > 2 {
			execCalled <- cmd[2]
		}
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}
}
