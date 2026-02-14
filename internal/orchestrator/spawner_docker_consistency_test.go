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

func TestDockerSpawner_GitConfigInjection(t *testing.T) {
	// Set GITHUB_TOKEN environment variable for the test
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	// We pass nil for SessionManager because we mock its calls
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)

	// Mock GitClient
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TASK-GIT-CONFIG",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-git-config", nil)
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)
	// The final SHA call might happen
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil).Maybe()


	// Capture the command passed to Exec
	capturedCmdChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-git-config", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedCmdChan <- capturedCmd
	}).Return("Success", nil)

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	var capturedCmd []string
	select {
	case capturedCmd = <-capturedCmdChan:
		// Success
	case <-time.After(5 * time.Second): // Generous timeout for async goroutine
		t.Fatal("Timed out waiting for Exec call")
	}

	cmdStr := capturedCmd[2]

	// Verify the git config command is present
	expectedGitConfig := `git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"`
	// We check for the core part since spacing/newlines might vary
	assert.Contains(t, cmdStr, "git config --global url", "Command should contain git config setup")
	assert.Contains(t, cmdStr, "${GITHUB_TOKEN}", "Command should reference GITHUB_TOKEN")
	assert.Contains(t, cmdStr, expectedGitConfig, "Command should contain the full git config setup script")
}
