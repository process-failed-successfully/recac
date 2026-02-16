package orchestrator

import (
	"context"
	"log/slog"
	"os"
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
	sm.On("SaveSession", mock.Anything).Return(nil)

	done := make(chan struct{})
	var once sync.Once
	sm.On("LoadSession", mock.Anything).Run(func(args mock.Arguments) {
		once.Do(func() {
			close(done)
		})
	}).Return(nil, assert.AnError) // Return error to stop early

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
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	// Wait for LoadSession
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for LoadSession call")
	}

	cmdStr := capturedCmd[2]

	// We assert that the injection DID NOT happen (i.e., the string "echo PWNED" is NOT executable code).
	// In a safe implementation, the single quote should be escaped.

	// Check for the specific unsafe pattern: ='<payload>'
	// unsafe := "export MALICIOUS_VAR=''; echo PWNED; ''"

	// We want to FAIL if the output is unsafe.
	// The test should pass if the code is FIXED.

	// Construct the exact expected vulnerable string for that part.
	vulnerableSubstring := "export MALICIOUS_VAR=''; echo PWNED; ''"

	assert.NotContains(t, cmdStr, vulnerableSubstring, "Command string contains unescaped malicious payload!")
}
