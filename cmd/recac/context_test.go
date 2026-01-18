package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCmd(t *testing.T) {
	// Setup temp dir
	tempDir, err := os.MkdirTemp("", "recac-context-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create structure:
	// - file1.txt
	// - src/
	//   - main.go
	// - .git/
	//   - config (should be ignored)
	// - ignored_dir/
	//   - ignored.txt
	// - binary.bin (should be ignored)

	createFile(t, tempDir, "file1.txt", "Hello World")
	createDir(t, tempDir, "src")
	createFile(t, tempDir, "src/main.go", "package main")
	createDir(t, tempDir, ".git")
	createFile(t, tempDir, ".git/config", "secret")
	createDir(t, tempDir, "ignored_dir")
	createFile(t, tempDir, "ignored_dir/ignored.txt", "ignore me")
	createFile(t, tempDir, "binary.bin", string([]byte{0, 1, 2, 3})) // Null byte

	// Reset flags after test
	defer func() {
		ctxCopy = false
		ctxOutput = ""
		ctxTokens = false
		ctxTree = true
		ctxMaxSize = 1024 * 1024
		ctxIgnore = nil
		ctxNoContent = false
	}()

	t.Run("Basic", func(t *testing.T) {
		resetContextFlags()
		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// Set args to tempDir
		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "# File Tree")
		assert.Contains(t, output, "file1.txt")
		assert.Contains(t, output, "src")
		assert.Contains(t, output, "main.go")

		assert.Contains(t, output, "# File Contents")
		// We relax the path check because it might be absolute or relative depending on CWD
		assert.Contains(t, output, "file1.txt")
		assert.Contains(t, output, "Hello World")
		assert.Contains(t, output, "package main")

		// Assert ignores
		assert.NotContains(t, output, ".git")
		assert.NotContains(t, output, "secret")

		// binary.bin might be in tree, but should not be in content
		// We check that we don't start a file block for it
		assert.NotContains(t, output, "## File: binary.bin")
		// And if it had a path prefix
		assert.NotContains(t, output, "binary.bin\n\n```")
	})

	t.Run("IgnoreCustom", func(t *testing.T) {
		resetContextFlags()
		ctxIgnore = []string{"src"}

		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "file1.txt")
		assert.NotContains(t, output, "src")
		assert.NotContains(t, output, "main.go")
	})

	t.Run("IgnoreFile", func(t *testing.T) {
		resetContextFlags()
		ctxIgnore = []string{"file1.txt"}

		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		output := buf.String()
		// Should not be in tree or content
		assert.NotContains(t, output, "file1.txt")
		assert.NotContains(t, output, "Hello World")

		// Other files should exist
		assert.Contains(t, output, "main.go")
	})

	t.Run("NoContent", func(t *testing.T) {
		resetContextFlags()
		ctxNoContent = true

		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "# File Tree")
		assert.NotContains(t, output, "# File Contents")
		assert.NotContains(t, output, "Hello World")
	})

	t.Run("Tokens", func(t *testing.T) {
		resetContextFlags()
		ctxTokens = true

		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Estimated Tokens:")
	})

	t.Run("OutputFile", func(t *testing.T) {
		resetContextFlags()
		outFile := filepath.Join(tempDir, "context.md")
		ctxOutput = outFile

		cmd := contextCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		err := cmd.RunE(cmd, []string{tempDir})
		require.NoError(t, err)

		// Check file exists
		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "Hello World")

		// Stdout should be empty (except for success message)
		assert.Contains(t, buf.String(), "Context written to")
	})
}

func resetContextFlags() {
	ctxCopy = false
	ctxOutput = ""
	ctxTokens = false
	ctxTree = true
	ctxMaxSize = 1024 * 1024
	ctxIgnore = nil
	ctxNoContent = false
}

func createFile(t *testing.T, dir, path, content string) {
	fullPath := filepath.Join(dir, path)
	err := os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(t, err)
}

func createDir(t *testing.T, dir, path string) {
	fullPath := filepath.Join(dir, path)
	err := os.MkdirAll(fullPath, 0755)
	require.NoError(t, err)
}
