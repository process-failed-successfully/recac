package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	type execArgs struct {
		cmdStr      string
		containerID string
	}
	execCalled := make(chan execArgs, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	// This prevents the goroutine from exiting early if arguments don't match exactly (e.g. due to race conditions or strict matching)
	mockDocker.On("Exec", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		containerID := args.Get(1).(string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		if len(cmd) > 2 {
			execCalled <- execArgs{cmdStr: cmd[2], containerID: containerID}
		}
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case args := <-execCalled:
		t.Logf("Captured Command: %s", args.cmdStr)
		assert.Equal(t, "container123", args.containerID, "Exec should be called on the correct container")
		assert.Contains(t, args.cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, args.cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}
}
