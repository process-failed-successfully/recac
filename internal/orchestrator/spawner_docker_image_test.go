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

	done := make(chan struct{})

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// We use Run to signal that the background goroutine has reached this point.
	// Since we return an error here, the goroutine will exit after this call,
	// making it the final synchronization point for this test.
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil, assert.AnError)

	// Use MatchedBy to verify the command arguments in-place
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2] // /bin/sh -c <cmdStr>
		// Log for debugging if test fails
		t.Logf("Captured Command: %s", cmdStr)

		containsImageFlag := assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		containsImageName := assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")

		return containsImageFlag && containsImageName
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-done:
		// Success: Goroutine reached LoadSession and will exit shortly.
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call (goroutine completion)")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
