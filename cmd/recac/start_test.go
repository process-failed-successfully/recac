package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/docker"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestStartCommand_Detached(t *testing.T) {
	// Setup Mock SessionManager
	mockSM := NewMockSessionManager()

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	tmpDir := t.TempDir()

	// Execute start --detached --name test-session --path tmpDir --mock
	var err error
	output := captureOutput(func() {
		_, err = executeCommand(rootCmd, "start",
			"--detached",
			"--name", "test-session",
			"--path", tmpDir,
			"--mock",
		)
	})

	// Verify output
	// executeCommand catches exit(1) but detached shouldn't exit 1.
	require.NoError(t, err)
	assert.Contains(t, output, "Session 'test-session' started in background")

	// Verify SessionManager called
	if assert.Contains(t, mockSM.Sessions, "test-session") {
		session := mockSM.Sessions["test-session"]
		assert.Equal(t, "test-session", session.Name)
		assert.Equal(t, tmpDir, session.Workspace)
	}
}

func TestStartCommand_MockMode_Interactive(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	var err error
	output := captureOutput(func() {
		_, err = executeCommand(rootCmd, "start",
			"--mock",
			"--path", tmpDir,
			"--max-iterations", "1",
			"--name", "interactive-test",
		)
	})

	if err != nil {
		t.Logf("Command failed with output: %s", output)
		// If error is "maximum iterations reached", treat as success for this test
		// as we are testing the startup, not full completion.
		if strings.Contains(err.Error(), "maximum iterations reached") {
			err = nil
		}
	}
	require.NoError(t, err)
	assert.Contains(t, output, "Starting in MOCK MODE")
}

func TestStartCommand_Resume(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	t.Setenv("HOME", t.TempDir())

	output := captureOutput(func() {
		executeCommand(rootCmd, "start",
			"--resume-from", tmpDir,
			"--mock",
			"--max-iterations", "1",
			"--name", "resume-test",
		)
	})

	// Just check output
	assert.Contains(t, output, fmt.Sprintf("Resuming session 'resume-test' from workspace: %s", tmpDir))
}

func TestStartCommand_NormalMode_Restricted(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Mock agentClientFactory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Mock docker.NewClient to return error, forcing restricted mode
	originalDockerFactory := newDockerClientFunc
	newDockerClientFunc = func(projectName string) (*docker.Client, error) {
		return nil, fmt.Errorf("mock docker failure")
	}
	defer func() { newDockerClientFunc = originalDockerFactory }()

	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		_, err = executeCommand(rootCmd, "start",
			"--path", tmpDir,
			"--max-iterations", "1",
			"--name", "normal-test",
			"--allow-dirty",
			"--project", "test-project",
		)
	})

	// We expect "maximum iterations reached" error because max-iterations=1
	// and StartSession returns error in that case.
	// But `executeCommand` returns error.
	// The original test expected Success because exit(1) was swallowed.
	// Now we expect an error, specifically "maximum iterations reached" or similar.
	// Actually, runner returns ErrMaxIterations.
	// Let's adjust expectation.
	// Wait, if max-iterations is reached, is it considered an error for the CLI?
	// CLI usually prints "reached max iterations" and exits 0?
	// In `start.go`: return runErr.
	// RunLoop returns error on max iterations.
	// So CLI returns error.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum iterations reached")
	assert.Contains(t, output, "Starting RECAC session")
	assert.Contains(t, output, "Proceeding in restricted mode")
}
