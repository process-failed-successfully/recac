package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"
	"os/exec"

	"github.com/stretchr/testify/assert"
)

type ResolveSpyAgent struct {
	Response string
}

func (s *ResolveSpyAgent) Send(ctx context.Context, prompt string) (string, error) {
	return s.Response, nil
}

func (s *ResolveSpyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return s.Response, nil
}

func TestResolveCommand(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &ResolveSpyAgent{Response: "Resolved Code"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Test Case 1: Resolve specific file
	t.Run("Resolve specific file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "conflict.txt")
		content := `Before
<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch
After`
		err := os.WriteFile(filePath, []byte(content), 0644)
		assert.NoError(t, err)

		// We need to invoke runResolve directly or via Execute.
		// Since resolveCmd is global and has flags, we need to be careful.
		// Let's call runResolve directly to avoid flag parsing issues if possible,
		// but runResolve takes *cobra.Command.

		// Reset flags
		resolveCmd.Flags().Set("auto", "true")

		// Redirect stdout/stderr to suppress output
		oldStdout := resolveCmd.OutOrStdout()
		resolveCmd.SetOut(io.Discard)
		defer resolveCmd.SetOut(oldStdout)

		err = runResolve(resolveCmd, []string{filePath})
		assert.NoError(t, err)

		// Verify content
		resolvedContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		expected := "Before\nResolved CodeAfter"
		assert.Equal(t, expected, string(resolvedContent))
	})

	// Test Case 2: Parse Conflict Block 3-way
	t.Run("Parse 3-way conflict", func(t *testing.T) {
		block := `<<<<<<< HEAD
Ours
||||||| merged common ancestors
Base
=======
Theirs
>>>>>>> branch`
		ours, theirs, err := parseConflictBlock(block)
		assert.NoError(t, err)
		assert.Equal(t, "Ours", ours)
		assert.Equal(t, "Theirs", theirs)
	})

	// Test Case 3: Parse Conflict Block 2-way
	t.Run("Parse 2-way conflict", func(t *testing.T) {
		block := `<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch`
		ours, theirs, err := parseConflictBlock(block)
		assert.NoError(t, err)
		assert.Equal(t, "Ours", ours)
		assert.Equal(t, "Theirs", theirs)
	})
}

func TestFindConflictedFiles(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(command string, args ...string) *exec.Cmd {
		// Mock git grep
		if command == "git" && args[0] == "grep" {
			return exec.Command("echo", "file1.txt\nfile2.txt")
		}
		return exec.Command(command, args...)
	}

	files, err := findConflictedFiles()
	assert.NoError(t, err)
	assert.Equal(t, []string{"file1.txt", "file2.txt"}, files)
}

func TestConfirm(t *testing.T) {
	// Redirect os.Stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("y\n"))
		w.Close()
	}()

	result := confirm("Are you sure?")
	assert.True(t, result)
}

func TestConfirm_No(t *testing.T) {
	// Redirect os.Stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("n\n"))
		w.Close()
	}()

	result := confirm("Are you sure?")
	assert.False(t, result)
}

func TestRunResolve_NoConflicts(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(command string, args ...string) *exec.Cmd {
		// Mock git grep returning no matches
		if command == "git" && args[0] == "grep" {
			cmd := exec.Command("sh", "-c", "exit 1")
			return cmd
		}
		return exec.Command(command, args...)
	}

	// Capture stdout
	oldStdout := resolveCmd.OutOrStdout()
	resolveCmd.SetOut(io.Discard)
	defer resolveCmd.SetOut(oldStdout)

	err := runResolve(resolveCmd, []string{})
	assert.NoError(t, err)
}

func TestRunResolve_NotAuto(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &ResolveSpyAgent{Response: "Resolved Code"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "conflict_noauto.txt")
	content := `Before
<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch
After`
	err := os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err)

	resolveCmd.Flags().Set("auto", "false")

	// Redirect os.Stdin to decline
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		w.Write([]byte("n\n"))
		w.Close()
	}()

	oldStdout := resolveCmd.OutOrStdout()
	resolveCmd.SetOut(io.Discard)
	defer resolveCmd.SetOut(oldStdout)

	err = runResolve(resolveCmd, []string{filePath})
	assert.NoError(t, err)

	// Verify content is unchanged since we declined
	resolvedContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)

	assert.Equal(t, content, string(resolvedContent))
}
