package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
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

	// 1. Initial SaveSession (Status: running)
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// 2. Exec expectation
	execCalled := make(chan string, 1)
	var execOnce sync.Once
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		execOnce.Do(func() {
			cmd := args.Get(2).([]string)
			// cmd is ["/bin/sh", "-c", "actual command"]
			if len(cmd) > 2 {
				execCalled <- cmd[2]
			}
		})
	}).Return("output", nil)

	// 3. LoadSession (called in background after Exec)
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Name:      "TICKET-1",
		Status:    "running",
		Workspace: "/tmp/workspace",
	}, nil)

	// 4. Git SHA (called in background)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha123", nil).Maybe()

	// 5. Final SaveSession (Status: completed) - used to synchronize test end
	done := make(chan struct{})
	var doneOnce sync.Once
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed"
	})).Run(func(args mock.Arguments) {
		doneOnce.Do(func() {
			close(done)
		})
	}).Return(nil)

	// RunContainer expectation
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// Execute Spawn
	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// Verify Exec call with increased timeout
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for background goroutine to complete
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for background goroutine completion")
	}
}
