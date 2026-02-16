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

	execDone := make(chan struct{})
	var execOnce sync.Once

	// We match arguments using MatchedBy
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		// Check for image flag
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	})).Run(func(args mock.Arguments) {
		execOnce.Do(func() {
			close(execDone)
		})
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-execDone:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call or command verification failed")
	}

	// Wait for background goroutine to reach LoadSession to ensure clean shutdown of mocks
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Log("Timeout waiting for LoadSession (goroutine cleanup)")
	}
}
