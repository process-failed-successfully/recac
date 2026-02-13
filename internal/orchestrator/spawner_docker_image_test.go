package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

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

	// Synchronization channel to ensure goroutine completes
	done := make(chan bool)

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// Initial SaveSession
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// LoadSession called inside goroutine
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{}, nil)

	// Exec called inside goroutine - Verify --image flag here
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		// cmd is ["/bin/sh", "-c", "actual command"]
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	})).Return("output", nil)

	// CurrentCommitSHA called inside goroutine
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha123", nil)

	// Final SaveSession - Use this to signal completion
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed"
	})).Return(nil).Run(func(args mock.Arguments) {
		close(done)
	})

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err, "Spawn should not return error")

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for goroutine to complete")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
