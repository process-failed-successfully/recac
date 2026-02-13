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

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// Initial SaveSession (Status: running)
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// Final SaveSession (Status: completed/error) - Synchronization hook
	done := make(chan struct{})
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" || s.Status == "error"
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil)

	// LoadSession called by goroutine (should succeed now)
	dummySession := &runner.SessionState{
		Name:      "TICKET-1",
		Status:    "running",
		Workspace: "/tmp/workspace", // Dummy workspace
	}
	mockSM.On("LoadSession", "TICKET-1").Return(dummySession, nil)

	// Clean up mock call (since LoadSession succeeds, we might have git SHA calls if applicable)
	// But in DockerSpawner logic:
	// 7. Get end commit SHA
	// endSHA, shaErr := s.GitClient.CurrentCommitSHA(tempDir)
	// If GitClient is used, we need to mock it or it will be called.
	// spawner.GitClient is set to mockGit.
	// We need to expect CurrentCommitSHA.
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha123", nil).Maybe()

	execCalled := make(chan string, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Wait for Exec call first
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for background goroutine completion (Final SaveSession)
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for background goroutine to complete")
	}
}
