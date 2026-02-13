package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
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

	// 1. Mock RunContainer
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// 2. Mock Initial SaveSession (Status=running)
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil).Once()

	// 3. Mock Exec (Capture command)
	execCalled := make(chan string, 1)
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	// 4. Mock LoadSession (Return success, Status=running)
	// This session state is used as the base for the final update.
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Status: "running",
	}, nil)

	// 5. Mock CurrentCommitSHA
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	// 6. Mock Final SaveSession (Status=completed) -> Signal done
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed"
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil)

	// Run Spawn
	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Verify Exec called with correct flags
	select {
	case cmdStr := <-execCalled:
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Verify full execution completed
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession call (goroutine completion)")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
