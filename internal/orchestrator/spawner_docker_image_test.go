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
	// Use mock.Anything for context to be safe against context wrapping/modification
	mockDocker.On("RunContainer", mock.Anything, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	execCalled := make(chan string, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err) // Use require.NoError to fail fast

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(5 * time.Second): // Increase timeout for CI stability
		t.Fatal("Timeout waiting for Exec call")
	}
}
