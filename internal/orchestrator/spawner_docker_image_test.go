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

	done := make(chan struct{})
	var once sync.Once

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	// Synchronize on LoadSession - this ensures the goroutine has completed the critical section (Exec)
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		once.Do(func() {
			close(done)
		})
	}).Return(nil, assert.AnError)

	// Verify the --image flag is passed correctly using MatchedBy
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		fullCmd := strings.Join(cmd, " ")
		return strings.Contains(fullCmd, "--image") && strings.Contains(fullCmd, imageName)
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
