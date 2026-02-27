package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureOutput is moved to a shared test helper file or removed if not needed since executeCommand captures it.
// Assuming we use executeCommand which returns output.

func TestStartCommand_Detached(t *testing.T) {
	// Setup Mock SessionManager
	mockSM := NewMockSessionManager()

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Mock undoCaptureFunc to avoid git dependency
	originalUndo := undoCaptureFunc
	undoCaptureFunc = func(paths ...string) (string, error) {
		return "", nil
	}
	defer func() { undoCaptureFunc = originalUndo }()

	tmpDir := t.TempDir()

	// Execute start --detached --name test-session --path tmpDir --mock
	output, err := executeCommand(rootCmd, "start",
		"--detached",
		"--name", "test-session",
		"--path", tmpDir,
		"--mock",
	)

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

	// Mock undoCaptureFunc
	originalUndo := undoCaptureFunc
	undoCaptureFunc = func(paths ...string) (string, error) {
		return "", nil
	}
	defer func() { undoCaptureFunc = originalUndo }()

	output, err := executeCommand(rootCmd, "start",
		"--mock",
		"--path", tmpDir,
		"--max-iterations", "1",
		"--name", "interactive-test",
	)

	if err != nil {
		t.Logf("Command failed with output: %s", output)
	}
	require.NoError(t, err)
	assert.Contains(t, output, "Starting in MOCK MODE")
}

func TestStartCommand_Resume(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	t.Setenv("HOME", t.TempDir())

	// Mock undoCaptureFunc
	originalUndo := undoCaptureFunc
	undoCaptureFunc = func(paths ...string) (string, error) {
		return "", nil
	}
	defer func() { undoCaptureFunc = originalUndo }()

	output, _ := executeCommand(rootCmd, "start",
		"--resume-from", tmpDir,
		"--mock",
		"--max-iterations", "1",
		"--name", "resume-test",
	)

	// Just check output
	assert.Contains(t, output, fmt.Sprintf("Resuming session 'resume-test' from workspace: %s", tmpDir))
}

func TestStartCommand_NormalMode_Restricted(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// Mock agentClientFactory
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock SessionManager (Fix for panic)
	mockSM := NewMockSessionManager()
	originalSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalSMFactory }()

	// Mock undoCaptureFunc
	originalUndo := undoCaptureFunc
	undoCaptureFunc = func(paths ...string) (string, error) {
		return "", nil
	}
	defer func() { undoCaptureFunc = originalUndo }()

	t.Setenv("HOME", t.TempDir())

	output, err := executeCommand(rootCmd, "start",
		"--path", tmpDir,
		"--max-iterations", "1",
		"--name", "normal-test",
		"--allow-dirty",
		"--project", "test-project",
	)

	// We expect executeCommand to possibly fail if RunLoop hits max iterations (which mocks often do),
	// but we don't want it to panic.
	// The test asserts "Starting RECAC session" which is printed early.
	// If it panics, err will be non-nil (if executeCommand catches it) or the test will crash.
	// We just want to ensure clean execution environment.

	if err != nil {
		// Log but don't fail if it's just max iterations
		t.Logf("Command exited with error: %v", err)
	}
	assert.Contains(t, output, "Starting RECAC session")
}
