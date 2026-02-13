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

	// Mock expectations
	// 1. Start container
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// 2. Initial SaveSession (start)
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running" && s.ContainerID == "container123"
	})).Return(nil)

	// 3. Exec call
	execCalled := make(chan string, 1)
	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	// 4. LoadSession for update
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Name:      "TICKET-1",
		Status:    "running",
		Workspace: "/tmp/mock-workspace",
	}, nil)

	// 5. Get End SHA (called if LoadSession succeeds)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("mock-sha", nil)

	// 6. Final SaveSession (completion)
	// We use Run to signal that the background goroutine has reached this point.
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" && s.EndCommitSHA == "mock-sha"
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Verify Exec command
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Verify full completion
	select {
	case <-done:
		// Success: Goroutine reached final SaveSession.
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession call (goroutine completion)")
	}
}
