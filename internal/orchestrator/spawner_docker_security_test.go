package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"recac/internal/runner"
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
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)

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
	// Initial SaveSession
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// Final SaveSession
	done := make(chan struct{})
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status != "running"
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil)

	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

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

	// Wait for the background goroutine to complete
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for final SaveSession")
	}

	// The command should be stringified and passed to sh -c.
	// We want to ensure the command string is SAFE.
	cmdStr := capturedCmd[2]

	// Construct the exact expected vulnerable string for that part.
	vulnerableSubstring := "export MALICIOUS_VAR=''; echo PWNED; ''"

	assert.NotContains(t, cmdStr, vulnerableSubstring, "Command string contains unescaped malicious payload!")
}
