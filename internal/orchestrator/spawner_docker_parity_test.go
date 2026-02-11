package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestDockerSpawner_GitConfigParity verifies that DockerSpawner injects git config for GITHUB_TOKEN
// ensuring parity with K8sSpawner.
func TestDockerSpawner_GitConfigParity(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)

	// Mock Environment
	os.Setenv("GITHUB_TOKEN", "mock-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	item := WorkItem{
		ID:      "TASK-PARITY-TEST",
		RepoURL: "https://github.com/example/repo",
	}

	// Mock Expectations
	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-parity", nil)
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	// Capture the command passed to Exec
	capturedCmdChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-parity", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedCmdChan <- capturedCmd
	}).Return("Success", nil)

	// Execute
	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Wait for Exec
	var capturedCmd []string
	select {
	case capturedCmd = <-capturedCmdChan:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	cmdStr := capturedCmd[2]

	// Assert: Check for git config logic parity
	// K8sSpawner uses:
	// if [ -n "$GITHUB_TOKEN" ]; then
	//     git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
	// fi

	// We check for the key parts of this command
	assert.Contains(t, cmdStr, "git config --global url.", "Should contain git config command")
	assert.Contains(t, cmdStr, "${GITHUB_TOKEN}", "Should use GITHUB_TOKEN environment variable")
	assert.Contains(t, cmdStr, "x-oauth-basic@github.com/", "Should use x-oauth-basic auth format")
}
