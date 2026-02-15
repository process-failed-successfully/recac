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
	// Use mock.Anything for context to ensure it matches even if wrapped
	mockDocker.On("RunContainer", mock.Anything, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	execCalled := make(chan string, 1)

	// Use mock.Anything for containerID as well, to avoid mismatch if something weird happens with string passing
	mockDocker.On("Exec", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		containerID := args.String(1)
		if containerID != "container123" {
			t.Logf("Warning: Expected containerID 'container123', got '%s'", containerID)
		}
		cmd := args.Get(2).([]string)
		if len(cmd) > 2 {
			execCalled <- cmd[2]
		} else {
			t.Logf("Warning: Expected cmd length > 2, got %d", len(cmd))
			execCalled <- ""
		}
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		if cmdStr != "" {
			assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
			assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
		} else {
			t.Error("Received empty command string")
		}
	case <-time.After(60 * time.Second): // Increased timeout
		t.Fatal("Timeout waiting for Exec call")
	}
}
