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
	// Using mock.Anything for most args as we are testing Image flag propagation primarily
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// Use a done channel to signal completion of the background goroutine
	done := make(chan struct{})

	// LoadSession is called AFTER Exec, so it's a good place to signal completion
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(done)
	}).Return((*runner.SessionState)(nil), assert.AnError)

	// Verify Exec call directly with MatchedBy
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		// cmd is ["/bin/sh", "-c", "actual command"]
		cmdStr := cmd[2]

		containsImage := strings.Contains(cmdStr, "--image")
		containsImageName := strings.Contains(cmdStr, imageName)

		if !containsImage {
			t.Logf("Command missing --image flag: %s", cmdStr)
		}
		if !containsImageName {
			t.Logf("Command missing image name: %s", cmdStr)
		}

		return containsImage && containsImageName
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for background goroutine completion (Exec -> LoadSession)")
	}

	// Ensure expectations were met
	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
