package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvolution_WorktreeFailure(t *testing.T) {
	// 1. Setup mock execCommand to simulate failure for "worktree add"
	originalExec := execCommand
	defer func() { execCommand = originalExec }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		// Mock git log (succeeds)
		if len(args) > 0 && args[0] == "log" {
			// Return 1 commit
			return exec.Command("echo", "hash12345 2023-01-01")
		}
		// Mock git worktree add (fails)
		if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			return exec.Command("false")
		}
		// Other commands (cleanup) succeed
		return exec.Command("true")
	}

	// We must pass a command object because runEvolutionAnalysis writes to it
	cmd := evolutionCmd
	// Set dummy output to avoid panic
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	metrics, err := runEvolutionAnalysis(cmd, ".", 30)

	// 3. Verify
	// It should not fail completely, but return empty metrics (or whatever was successfully analyzed)
	require.NoError(t, err)
	assert.Len(t, metrics, 0)
}

func TestEvolution_JSON(t *testing.T) {
	// 1. Setup real repo
	repoDir := t.TempDir()

	// Helper to run git
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Run()
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")

	// Commit
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main(){}"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "Init")

	// Set Flag
	evolutionJSON = true
	defer func() { evolutionJSON = false }()

	// Run
	// We need to call evolutionCmd.RunE to test JSON output logic
	// But first ensure runEvolutionAnalysis works with real git (no mock)
	execCommand = exec.Command

	// We need to pass args to RunE. It takes cmd and args.
	// We can manually set the output buffer of cmd.
	cmd := evolutionCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := evolutionCmd.RunE(cmd, []string{repoDir})
	require.NoError(t, err)

	output := buf.String()
	// Should be JSON array
	assert.Contains(t, output, "[")
	assert.Contains(t, output, "date")
	assert.Contains(t, output, "loc")
}
