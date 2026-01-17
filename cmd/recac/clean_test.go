package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanCmd(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "recac-clean-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to tmpDir so `clean` command finds `temp_files.txt`
	originalWd, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	t.Run("Clean No Files", func(t *testing.T) {
		cmd := NewCleanCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)

		cmd.Execute()
		assert.Contains(t, b.String(), "No temporary files to clean.")
	})

	t.Run("Clean Files", func(t *testing.T) {
		// Create a temporary file to clean
		fileToClean := "to_delete.txt"
		err := os.WriteFile(fileToClean, []byte("content"), 0644)
		require.NoError(t, err)

		// Create temp_files.txt listing the file
		err = os.WriteFile("temp_files.txt", []byte(fileToClean+"\n"), 0644)
		require.NoError(t, err)

		cmd := NewCleanCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)

		cmd.Execute()
		assert.Contains(t, b.String(), "Cleaning up temporary files...")
		// Use filepath.Abs because the command resolves absolute path
		absPath, _ := filepath.Abs(fileToClean)
		assert.Contains(t, b.String(), "Removed "+absPath)
		assert.Contains(t, b.String(), "Cleanup complete.")

		// Verify file is gone
		_, err = os.Stat(fileToClean)
		assert.True(t, os.IsNotExist(err), "File should be deleted")

		// Verify temp_files.txt is gone
		_, err = os.Stat("temp_files.txt")
		assert.True(t, os.IsNotExist(err), "temp_files.txt should be deleted")
	})
}
