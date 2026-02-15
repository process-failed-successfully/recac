package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
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

	// Use a channel to signal when LoadSession is called
	loadSessionCalled := make(chan struct{})

	// We expect LoadSession to be called once by the background goroutine.
	// We return an error to stop the goroutine from proceeding further (e.g. to SaveSession again).
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(loadSessionCalled)
	}).Return(&runner.SessionState{}, assert.AnError)

	execCalled := make(chan struct{})

	// Verify Exec is called with the image flag
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		// Check for --image flag and value
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	}), mock.Anything).Run(func(args mock.Arguments) {
		close(execCalled)
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Wait for Exec call
	select {
	case <-execCalled:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for the background goroutine to call LoadSession
	select {
	case <-loadSessionCalled:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call")
	}

	// Give the background goroutine time to complete (cleanup, final save, etc.)
	time.Sleep(100 * time.Millisecond)
}
