package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"strings"
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Synchronization: we wait for the FINAL SaveSession call
	done := make(chan struct{})
	var once sync.Once

	// 1. RunContainer
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// 2. Initial SaveSession
	// Matches any session state, returns nil
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// 3. Exec
	// Verify --image flag here using MatchedBy
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2] // /bin/sh -c <cmdStr>
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	})).Return("output", nil)

	// 4. LoadSession
	// Return a valid session to allow flow to continue
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Name:   "TICKET-1",
		Status: "running",
	}, nil)

	// 5. CurrentCommitSHA (called because flow continues)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("sha123", nil)

	// 6. Final SaveSession
	// This signals completion
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" && s.EndCommitSHA == "sha123"
	})).Run(func(_ mock.Arguments) {
		once.Do(func() {
			close(done)
		})
	}).Return(nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession call")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
