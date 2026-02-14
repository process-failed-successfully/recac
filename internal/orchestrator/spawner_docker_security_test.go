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
)

func TestDockerSpawner_EnvInjection_Vulnerability(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	mockGit := new(MockGitClient)
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
	spawner.GitClient = mockGit

	// Inject a malicious payload that tries to break out of single quotes
	// The payload ' closes the opening quote, then executes echo PWNED
	maliciousPayload := "'; echo PWNED; '"

	injectionItem := WorkItem{
		ID:      "TASK-SEC-1",
		RepoURL: "https://github.com/example/repo",
		EnvVars: map[string]string{
			"MALICIOUS_VAR": maliciousPayload,
		},
	}

	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-sec", nil)

	// Mock SessionManager
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	done := make(chan struct{})
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" || s.Status == "error"
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil).Once()

	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{Name: "TASK-SEC-1", Status: "running"}, nil)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	// Capture the command passed to Exec using a channel for synchronization
	capturedCmdChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-sec", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedCmdChan <- capturedCmd
	}).Return("Success", nil)

	err := spawner.Spawn(context.Background(), injectionItem)
	assert.NoError(t, err)

	// Wait for the background goroutine to call Exec
	var capturedCmd []string
	select {
	case capturedCmd = <-capturedCmdChan:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	// Verify completion
	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession (goroutine completion)")
	}

	cmdStr := capturedCmd[2]

	vulnerableSubstring := "export MALICIOUS_VAR=''; echo PWNED; ''"
	assert.NotContains(t, cmdStr, vulnerableSubstring, "Command string contains unescaped malicious payload!")
}
