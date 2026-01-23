package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"recac/internal/agent"

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

// TestResolveHelperProcess is a helper process for mocking exec.Command
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

	cmd, subArgs := args[0], args[1:]

	if cmd == "git" && len(subArgs) > 0 && subArgs[0] == "grep" {
		if os.Getenv("TEST_CASE") == "find_conflicted" {
			fmt.Println("file1.txt")
			fmt.Println("file2.txt")
		} else if os.Getenv("TEST_CASE") == "find_conflicted_empty" {
			// emulate exit code 1 for no matches
			os.Exit(1)
		} else if os.Getenv("TEST_CASE") == "find_conflicted_dynamic" {
			fmt.Println(os.Getenv("FILE_1"))
			fmt.Println(os.Getenv("FILE_2"))
		}
		return
	}
	os.Exit(1)
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
		resetFlags(resolveCmd)
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

	// Test Case 4: Find Conflicted Files
	t.Run("Find Conflicted Files", func(t *testing.T) {
		originalExec := execCommand
		defer func() { execCommand = originalExec }()

		execCommand = func(name string, arg ...string) *exec.Cmd {
			cs := []string{"-test.run=TestResolveHelperProcess", "--", name}
			cs = append(cs, arg...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "TEST_CASE=find_conflicted"}
			return cmd
		}

		files, err := findConflictedFiles()
		assert.NoError(t, err)
		assert.Equal(t, []string{"file1.txt", "file2.txt"}, files)
	})

	// Test Case 5: Interactive Confirmation (Yes)
	t.Run("Interactive Confirmation Yes", func(t *testing.T) {
		resetFlags(resolveCmd)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "conflict.txt")
		content := `<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch`
		err := os.WriteFile(filePath, []byte(content), 0644)
		assert.NoError(t, err)

		resolveCmd.Flags().Set("auto", "false")

		// Redirect input to simulate "y"
		inBuf := bytes.NewBufferString("y\n")
		resolveCmd.SetIn(inBuf)
		resolveCmd.SetOut(io.Discard)

		err = runResolve(resolveCmd, []string{filePath})
		assert.NoError(t, err)

		resolvedContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "Resolved Code", string(resolvedContent))
	})

	// Test Case 6: Interactive Confirmation (No)
	t.Run("Interactive Confirmation No", func(t *testing.T) {
		resetFlags(resolveCmd)
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "conflict.txt")
		content := `<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch`
		err := os.WriteFile(filePath, []byte(content), 0644)
		assert.NoError(t, err)

		resolveCmd.Flags().Set("auto", "false")

		// Redirect input to simulate "n"
		inBuf := bytes.NewBufferString("n\n")
		resolveCmd.SetIn(inBuf)
		resolveCmd.SetOut(io.Discard)

		err = runResolve(resolveCmd, []string{filePath})
		assert.NoError(t, err)

		// Content should remain unchanged
		resolvedContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, content, string(resolvedContent))
	})

	// Test Case 7: Run Resolve No Args
	t.Run("Run Resolve No Args", func(t *testing.T) {
		resetFlags(resolveCmd)
		tmpDir := t.TempDir()
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")

		// Create files
		content := "<<<<<<< HEAD\nA\n=======\nB\n>>>>>>>"
		os.WriteFile(file1, []byte(content), 0644)
		os.WriteFile(file2, []byte(content), 0644)

		originalExec := execCommand
		defer func() { execCommand = originalExec }()

		execCommand = func(name string, arg ...string) *exec.Cmd {
			cs := []string{"-test.run=TestResolveHelperProcess", "--", name}
			cs = append(cs, arg...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = []string{
				"GO_WANT_HELPER_PROCESS=1",
				"TEST_CASE=find_conflicted_dynamic",
				"FILE_1=" + file1,
				"FILE_2=" + file2,
			}
			return cmd
		}

		resolveCmd.Flags().Set("auto", "true")
		resolveCmd.SetOut(io.Discard)

		err := runResolve(resolveCmd, []string{})
		assert.NoError(t, err)

		// Verify they were resolved
		c1, _ := os.ReadFile(file1)
		assert.Contains(t, string(c1), "Resolved Code")
	})

	// Test Case 8: Find Conflicted Files Empty
	t.Run("Find Conflicted Files Empty", func(t *testing.T) {
		originalExec := execCommand
		defer func() { execCommand = originalExec }()

		execCommand = func(name string, arg ...string) *exec.Cmd {
			cs := []string{"-test.run=TestResolveHelperProcess", "--", name}
			cs = append(cs, arg...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "TEST_CASE=find_conflicted_empty"}
			return cmd
		}

		files, err := findConflictedFiles()
		assert.NoError(t, err)
		assert.Nil(t, files)
	})
}
