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
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	// Capture the command and env passed to Exec
	capturedCmdChan := make(chan []string, 1)
	capturedEnvChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-sec", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedEnv := args.Get(3).([]string)
		capturedCmdChan <- capturedCmd
		capturedEnvChan <- capturedEnv
	}).Return("Success", nil)

	err := spawner.Spawn(context.Background(), injectionItem)
	assert.NoError(t, err)

	// Wait for the background goroutine to call Exec
	var capturedCmd []string
	var capturedEnv []string
	select {
	case capturedCmd = <-capturedCmdChan:
		capturedEnv = <-capturedEnvChan
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	cmdStr := capturedCmd[2]

	// 1. Verify that the command string NO LONGER contains the exported environment variables
	assert.NotContains(t, cmdStr, "export MALICIOUS_VAR=", "Environment variables should not be in command string")

	// 2. Verify that the malicious payload is passed correctly in the Env slice
	// Docker handles env vars safely, so we check if the raw value is present in Env slice
	found := false
	expectedEnv := "MALICIOUS_VAR=" + maliciousPayload
	for _, env := range capturedEnv {
		if env == expectedEnv {
			found = true
			break
		}
	}
	assert.True(t, found, "Malicious payload should be passed raw in Env slice")
}
