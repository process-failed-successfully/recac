package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	// Ensure all expectations are met (specifically that Exec is called)
	defer mockDocker.AssertExpectations(t)
	defer mockSM.AssertExpectations(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Synchronization
	done := make(chan struct{})
	var once sync.Once

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// Close done channel when LoadSession is called.
	// Note: In DockerSpawner.Spawn, LoadSession is called inside the goroutine *after* Exec completes (or fails).
	// Synchronizing on LoadSession ensures that the goroutine has progressed past the Exec call, allowing
	// the Exec mock expectation (MatchedBy) to be validated fully before the test exits.
	// Since we mock LoadSession to return an error, the goroutine exits early here, making it the final call.
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		once.Do(func() {
			close(done)
		})
	}).Return(nil, assert.AnError)

	// Validate Exec arguments using MatchedBy
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		// cmd is ["/bin/sh", "-c", "actual command"]
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		return assert.Contains(t, cmdStr, "--image") && assert.Contains(t, cmdStr, imageName)
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call (and consequently Exec call)")
	}
}
