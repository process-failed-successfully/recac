package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"testing"

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
	// Mock execCommand to simulate git grep output
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	t.Run("Conflicts Found", func(t *testing.T) {
		execCommand = func(name string, arg ...string) *exec.Cmd {
			cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess_GitGrepConflicts")
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
			return cmd
		}
		files, err := findConflictedFiles()
		assert.NoError(t, err)
		assert.Equal(t, []string{"file1.txt", "file2.go"}, files)
	})

	t.Run("No Conflicts", func(t *testing.T) {
		execCommand = func(name string, arg ...string) *exec.Cmd {
			cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess_GitGrepNoConflicts")
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
			return cmd
		}
		files, err := findConflictedFiles()
		assert.NoError(t, err)
		assert.Empty(t, files)
	})
}

func TestConfirm(t *testing.T) {
	// Mock os.Stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	w.WriteString("y\n")
	w.Close()

	assert.True(t, confirm("test"))

	r, w, _ = os.Pipe()
	os.Stdin = r
	w.WriteString("no\n")
	w.Close()

	assert.False(t, confirm("test"))
}

func TestRunResolve_NoConflicts(t *testing.T) {
	// Mock execCommand to return no conflicts
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess_GitGrepNoConflicts")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	resolveCmd.Flags().Set("auto", "true")
	err := runResolve(resolveCmd, []string{})
	assert.NoError(t, err)
}
