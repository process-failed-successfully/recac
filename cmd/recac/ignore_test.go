package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIgnoreCmd(t *testing.T) {
	// Override rootCmd.Run to detect if it falls back to root (command not found)
	// and to prevent RunInteractive from hanging.
	originalRun := rootCmd.Run
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		t.Fatalf("rootCmd.Run called with args: %v. Ignore command was not dispatched!", args)
	}
	defer func() { rootCmd.Run = originalRun }()

	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-ignore-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Switch to temp dir so ignores operate there
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Use rootCmd and pass "ignore" as first arg
	t.Run("Add Pattern to .gitignore", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "node_modules")
		if err != nil {
			t.Logf("Error: %v", err)
		}

		assert.Contains(t, output, "Added 'node_modules' to .gitignore")

		content, _ := os.ReadFile(".gitignore")
		assert.Contains(t, string(content), "node_modules")
	})

	t.Run("Add Duplicate Pattern", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "node_modules")
		assert.NoError(t, err)
		assert.Contains(t, output, "Pattern 'node_modules' already exists")

		content, _ := os.ReadFile(".gitignore")
		count := strings.Count(string(content), "node_modules")
		assert.Equal(t, 1, count, "Should not duplicate pattern")
	})

	t.Run("Add Another Pattern", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "dist")
		assert.NoError(t, err)
		assert.Contains(t, output, "Added 'dist' to .gitignore")

		content, _ := os.ReadFile(".gitignore")
		assert.Contains(t, string(content), "node_modules\ndist\n")
	})

	t.Run("Remove Pattern", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "node_modules", "--remove")
		assert.NoError(t, err)
		assert.Contains(t, output, "Removed 'node_modules' from .gitignore")

		content, _ := os.ReadFile(".gitignore")
		assert.NotContains(t, string(content), "node_modules")
		assert.Contains(t, string(content), "dist")
	})

	t.Run("List Patterns", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "--list")
		assert.NoError(t, err)
		assert.Contains(t, output, "Contents of .gitignore:")
		assert.Contains(t, output, "dist")
	})

	t.Run("List with no args", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore")
		assert.NoError(t, err)
		assert.Contains(t, output, "Contents of .gitignore:")
		assert.Contains(t, output, "dist")
	})

	t.Run("Docker Ignore", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "secret.key", "--docker")
		assert.NoError(t, err)
		assert.Contains(t, output, "Added 'secret.key' to .dockerignore")

		content, _ := os.ReadFile(".dockerignore")
		assert.Contains(t, string(content), "secret.key")

		// Verify .gitignore was not touched (it should still have 'dist')
		gitContent, _ := os.ReadFile(".gitignore")
		assert.NotContains(t, string(gitContent), "secret.key")
		assert.Contains(t, string(gitContent), "dist")
	})

	t.Run("Remove Non-Existent Pattern", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "ignore", "non-existent", "--remove")
		assert.NoError(t, err)
		assert.Contains(t, output, "Pattern 'non-existent' not found")
	})

	t.Run("List Non-Existent File", func(t *testing.T) {
		os.Remove(".gitignore")
		output, err := executeCommand(rootCmd, "ignore", "--list")
		assert.NoError(t, err)
		assert.Contains(t, output, ".gitignore does not exist")
	})
}
