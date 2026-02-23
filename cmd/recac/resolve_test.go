package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper process for mocking execCommand
func TestResolveHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "git":
		if len(args) > 0 && args[0] == "grep" {
			// Output simulated file list
			fmt.Println("file1.txt")
			fmt.Println("file2.txt")
		}
	}
}

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
	// Mock execCommand
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestResolveHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommand = originalExecCommand }()

	files, err := findConflictedFiles()
	assert.NoError(t, err)
	assert.Contains(t, files, "file1.txt")
	assert.Contains(t, files, "file2.txt")
}
