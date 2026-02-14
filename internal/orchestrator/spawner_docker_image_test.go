package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"recac/internal/runner"

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

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// First SaveSession call (initial) - use specific matcher to differentiate
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil).Once()

	execCalled := make(chan string, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		t.Log("Exec called in mock")
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	// LoadSession called after Exec
	// Return a session with Name populated to avoid validation issues if mock was strict
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{Name: "TICKET-1", Status: "running"}, nil)

	// CurrentCommitSHA called after LoadSession
	// Return success to test happy path
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("sha123", nil)

	// Final SaveSession call - signals completion
	// Use specific matcher to ensure we catch the correct call
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" || s.Status == "error"
	})).Run(func(args mock.Arguments) {
		t.Log("SaveSession (final) called in mock")
		close(done)
	}).Return(nil).Once()

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Verify Exec call
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Verify completion
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession (goroutine completion)")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
