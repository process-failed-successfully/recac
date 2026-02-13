package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"strings"
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

	done := make(chan struct{})

	// Capture the command passed to Exec. We use mock.Anything for arguments to avoid strict matching
	// inside the goroutine, which can cause timeouts if the match fails silently (Run hook not called).
	var capturedCmd []string
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd = args.Get(2).([]string)
	}).Return("output", nil)

	// Synchronize on LoadSession, which is the final expected call in the goroutine when Exec succeeds (but LoadSession fails).
	// This ensures that Exec has completed before we assert on capturedCmd.
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil, assert.AnError)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-done:
		// Success: goroutine completed up to LoadSession
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call")
	}

	// Verify captured arguments in the main thread
	require.NotEmpty(t, capturedCmd, "Exec should have been called")
	fullCmd := strings.Join(capturedCmd, " ")
	assert.Contains(t, fullCmd, "--image", "Command should contain --image flag")
	assert.Contains(t, fullCmd, imageName, "Command should contain the correct image name")

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
