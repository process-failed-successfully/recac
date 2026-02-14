package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"recac/internal/runner"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_EnvInjection_Vulnerability(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	gitClient := new(MockGitClient)
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
	spawner.GitClient = gitClient

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

	done := make(chan struct{})
	var once sync.Once

	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-sec", nil)

	// Mock SessionManager
	// Initial SaveSession (synchronous, status=running)
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// LoadSession called in goroutine
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	// Mock GitClient used in goroutine
	gitClient.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

	// Final SaveSession (asynchronous, status=completed/error) - signal done here
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status != "running"
	})).Run(func(args mock.Arguments) {
		once.Do(func() { close(done) })
	}).Return(nil)

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
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	cmdStr := capturedCmd[2]

	// Check for the specific unsafe pattern: ='<payload>'
	vulnerableSubstring := "export MALICIOUS_VAR=''; echo PWNED; ''"

	assert.NotContains(t, cmdStr, vulnerableSubstring, "Command string contains unescaped malicious payload!")

	// Wait for goroutine to complete to avoid leaks
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for goroutine completion")
	}
}
