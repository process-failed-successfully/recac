package orchestrator

import (
	"context"
	"log/slog"
	"os"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// 1. Setup synchronization channels
	execCalled := make(chan string, 1)
	spawnDone := make(chan struct{})

	// 2. Define Mock Expectations
	// RunContainer should be called once with correct image
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil).Once()

	// SaveSession (Initial): Called synchronously in Spawn
	mockSM.On("SaveSession", mock.Anything).Return(nil).Once()

	// Exec: Called in goroutine
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		t.Log("Mock Exec called, processing arguments...")
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil).Once()

	// LoadSession: Called in goroutine after Exec
	// MUST return a valid session so the flow continues to the final SaveSession
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Status: "running",
	}, nil).Once()

	// Git CurrentCommitSHA: Called in goroutine
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha123", nil).Maybe()

	// SaveSession (Final): Called in goroutine at the end
	// Signals completion via spawnDone channel
	mockSM.On("SaveSession", mock.Anything).Run(func(args mock.Arguments) {
		t.Log("Final SaveSession called, signaling completion")
		close(spawnDone)
	}).Return(nil).Once()

	// 3. Execute Spawn
	t.Log("Starting Spawn")
	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	// 4. Verify Execution Command
	t.Log("Waiting for Exec")
	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(300 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// 5. Verify Cleanup/Completion
	t.Log("Waiting for Spawn completion")
	select {
	case <-spawnDone:
		t.Log("Spawn goroutine completed successfully")
	case <-time.After(300 * time.Second):
		t.Fatal("Timeout waiting for Spawn goroutine completion")
	}
}
