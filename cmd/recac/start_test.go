package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/workflow"
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

	// Override workflow.NewSessionManagerFunc
	originalFactory := workflow.NewSessionManagerFunc
	workflow.NewSessionManagerFunc = func() (workflow.ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { workflow.NewSessionManagerFunc = originalFactory }()

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

	// Mock cmdutils.GetAgentClient
	originalFactory := cmdutils.GetAgentClient
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return agent.NewMockAgent(), nil
	}
	defer func() { cmdutils.GetAgentClient = originalFactory }()

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

	require.NoError(t, err)
	assert.Contains(t, output, "Starting RECAC session")
}
