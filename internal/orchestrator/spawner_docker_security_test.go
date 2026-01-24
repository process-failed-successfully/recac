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

	// The command should be stringified and passed to sh -c.
	// We want to ensure the command string is SAFE.
	// Currently, it does fmt.Sprintf("export %s='%s'", k, v)
	// So it produces: export MALICIOUS_VAR=''; echo PWNED; ''
	// The shell will see:
	// 1. export MALICIOUS_VAR=''
	// 2. ; echo PWNED;
	// 3. ''

	cmdStr := capturedCmd[2]

	// We assert that the injection DID NOT happen (i.e., the string "echo PWNED" is NOT executable code).
	// In a safe implementation, the single quote should be escaped.
	// e.g. export MALICIOUS_VAR=''\'' echo PWNED; '\'''

	// If the vulnerability exists, we expect to see the raw sequence that closes the quote.
	// To verify failure, we check if the string contains the unsafe pattern.

	// Check for the specific unsafe pattern: ='<payload>'
	// unsafe := "export MALICIOUS_VAR=''; echo PWNED; ''"

	// We want to FAIL if the output is unsafe.
	// The test should pass if the code is FIXED.
	// So currently, this test should FAIL.

	// Use assert.NotContains to assert safety.
	// However, we want to prove it IS vulnerable first.
	// So I will check if it IS vulnerable, and fail if it IS.
	// But `go test` reports FAIL if assertions fail.

	// I want to write a test that passes when fixed.
	// So I will assert that the single quote IS escaped.

	// The escaped version of "'" is usually "'\''" in shell.
	// So we look for that pattern or verify that we don't find the raw payload surrounded by simple quotes.

	// Assert that the command string contains the payload ESCAPED.
	// Since we don't know the exact escaping mechanism yet, let's just assert that
	// we DON'T find the raw unescaped sequence which allows breakout.

	// Use regex or string search.
	// The vulnerable code produces: export MALICIOUS_VAR=''; echo PWNED; ''
	// The safe code produces something like: export MALICIOUS_VAR=''\'' echo PWNED; '\'''

	// We assert that we CANNOT find the sequence: =''; echo PWNED; ''
	// Actually, just checking that the single quote is escaped is enough.

	// Let's assert that the command string does NOT contain the raw payload as injected
	// because that would imply it wasn't escaped.
	// Wait, the payload is inside the string.

	// Let's check specifically for the closing quote that isn't escaped.
	// Construct the exact expected vulnerable string for that part.
	vulnerableSubstring := "export MALICIOUS_VAR=''; echo PWNED; ''"

	assert.NotContains(t, cmdStr, vulnerableSubstring, "Command string contains unescaped malicious payload!")

	// Also can assert that it IS escaped properly (once fixed)
	// assert.Contains(t, cmdStr, "'\\''")
}
