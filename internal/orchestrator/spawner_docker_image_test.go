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

	execCalled := make(chan string, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmdArg := args.Get(2)
		if cmdSlice, ok := cmdArg.([]string); ok && len(cmdSlice) > 2 {
			// cmd is ["/bin/sh", "-c", "actual command"]
			execCalled <- cmdSlice[2]
		}
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for background goroutine to reach LoadSession to ensure clean shutdown of mocks
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Log("Timeout waiting for LoadSession (goroutine cleanup)")
	}
}
