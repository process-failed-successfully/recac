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

	// Channels for synchronization
	execCalled := make(chan string, 1)
	done := make(chan struct{})

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// Mock Exec: capture the command and return success
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		if len(cmd) >= 3 {
			execCalled <- cmd[2]
		} else {
			execCalled <- ""
		}
	}).Return("output", nil)

	// Mock LoadSession: signal completion of the goroutine
	// This happens after Exec.
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil, assert.AnError) // Return error to stop further processing in the goroutine

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Verify Exec was called with correct arguments
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Verify the goroutine completed (reached LoadSession)
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call (goroutine completion)")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
