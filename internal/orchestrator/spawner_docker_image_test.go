package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
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
	// Initial container run
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// Initial session save
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// Synchronization for goroutine completion
	// When LoadSession is called (which happens after Exec in the goroutine), we signal completion
	done := make(chan struct{})
	var once sync.Once
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		once.Do(func() {
			close(done)
		})
	}).Return(nil, assert.AnError)

	// Use MatchedBy to verify the command arguments directly in the expectation
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		// cmd is ["/bin/sh", "-c", "actual command"]
		cmdStr := cmd[2]

		hasImageFlag := strings.Contains(cmdStr, "--image")
		hasImageName := strings.Contains(cmdStr, imageName)

		if !hasImageFlag || !hasImageName {
			t.Logf("Command missing image flag or name: %s", cmdStr)
		}

		return hasImageFlag && hasImageName
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Wait for background goroutine to reach LoadSession to ensure clean shutdown of mocks
	// and to ensure Exec was called (since LoadSession is called after Exec)
	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for LoadSession (goroutine cleanup)")
	}
}
