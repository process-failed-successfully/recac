package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisplayStatus(t *testing.T) {
	// Setup session and state
	session := &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		Goal:      "Implement feature X",
		StartTime: time.Now().Add(-10 * time.Minute),
	}
	state := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{
			TotalTokens:         1000,
			TotalPromptTokens:   500,
			TotalResponseTokens: 500,
		},
		History: []agent.Message{
			{
				Role:      "assistant",
				Content:   "I am working on it.",
				Timestamp: time.Now(),
			},
		},
	}

	// Capture output
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayStatus(cmd, session, state)

	output := buf.String()
	// tabwriter output uses tabs for alignment, assert.Contains handles substring search nicely
	// but spacing might be variable. We'll check for key phrases.
	assert.Contains(t, output, "Session:")
	assert.Contains(t, output, "test-session")
	assert.Contains(t, output, "Status:")
	assert.Contains(t, output, "\x1b[32mrunning\x1b[0m")
	assert.Contains(t, output, "Goal:")
	assert.Contains(t, output, "Implement feature X")
	assert.Contains(t, output, "Model:")
	assert.Contains(t, output, "gpt-4")
	assert.Contains(t, output, "Start Time:")
	assert.Contains(t, output, "Tokens:")
	assert.Contains(t, output, "1000")
	assert.Contains(t, output, "Est. Cost:")
	assert.Contains(t, output, "Content:")
	assert.Contains(t, output, "I am working on it.")
}

func TestDisplayStatus_LongGoal(t *testing.T) {
	// Setup session and state
	longGoal := strings.Repeat("a", 100)
	session := &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		Goal:      longGoal,
		StartTime: time.Now(),
	}
	state := &agent.State{
		Model: "gpt-4",
	}

	// Capture output
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayStatus(cmd, session, state)

	output := buf.String()
	assert.Contains(t, output, "Goal:")
	assert.Contains(t, output, longGoal[:57]+"...")
}

func TestPrintLogDiff_Success(t *testing.T) {
	// Mock diffExecCommand
	originalDiffExecCommand := diffExecCommand
	defer func() { diffExecCommand = originalDiffExecCommand }()

	// Case 1: Diff command returns exit code 1 (files differ)
	diffExecCommand = func(name string, arg ...string) *exec.Cmd {
		// Mock diff behavior: print diff and exit 1
		cmd := exec.Command("sh", "-c", "echo 'diff output'; exit 1")
		return cmd
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := printLogDiff(cmd, "file1.log", "file2.log")
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "diff output")
}

func TestPrintLogDiff_NoDiff(t *testing.T) {
	// Mock diffExecCommand
	originalDiffExecCommand := diffExecCommand
	defer func() { diffExecCommand = originalDiffExecCommand }()

	// Case 2: Diff command returns exit code 0 (files same)
	diffExecCommand = func(name string, arg ...string) *exec.Cmd {
		// Mock diff behavior: print nothing and exit 0
		cmd := exec.Command("true")
		return cmd
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := printLogDiff(cmd, "file1.log", "file2.log")
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No differences in logs.")
}

func TestPrintLogDiff_Fallback(t *testing.T) {
	// Mock diffExecCommand to fail (simulate missing command)
	originalDiffExecCommand := diffExecCommand
	defer func() { diffExecCommand = originalDiffExecCommand }()

	diffExecCommand = func(name string, arg ...string) *exec.Cmd {
		// Return a command that looks like it's not found
		// exec.Command itself doesn't check path existence until run, but LookPath does.
		// However, we mock the command creation.
		// If we want to trigger the specific error path `if _, ok := err.(*exec.Error); ok`,
		// we need `Run()` (called by `CombinedOutput`) to return `exec.Error`.
		// `exec.Command("nonexistentcommand")`'s Run() returns `exec.Error` if LookPath fails.
		return exec.Command("nonexistentcommand_for_test_12345")
	}

	// Create temporary files for fallback diff
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.log")
	file2 := filepath.Join(tmpDir, "file2.log")
	require.NoError(t, os.WriteFile(file1, []byte("line1\nline2"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("line1\nline3"), 0644))

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := printLogDiff(cmd, file1, file2)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "(using fallback diff)")
	// The path in output might contain extra spaces due to fmt.Println("--- ", file1)
	assert.Contains(t, output, "--- ")
	assert.Contains(t, output, file1)
	assert.Contains(t, output, "+++ ")
	assert.Contains(t, output, file2)
	assert.Contains(t, output, "- line2")
	assert.Contains(t, output, "+ line3")
}

func TestFallbackDiff(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.log")
	file2 := filepath.Join(tmpDir, "file2.log")

	// Same content
	require.NoError(t, os.WriteFile(file1, []byte("content"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content"), 0644))

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := fallbackDiff(cmd, file1, file2)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No differences in logs.")

	// Different content
	require.NoError(t, os.WriteFile(file2, []byte("different"), 0644))
	buf.Reset()

	err = fallbackDiff(cmd, file1, file2)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "- content")
	assert.Contains(t, buf.String(), "+ different")
}
