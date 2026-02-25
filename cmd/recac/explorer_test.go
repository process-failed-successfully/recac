package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunExplorer(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	// Mock startExplorerFunc
	origFunc := startExplorerFunc
	defer func() { startExplorerFunc = origFunc }()

	var capturedPath string
	startExplorerFunc = func(path string) error {
		capturedPath = path
		return nil
	}

	// 1. Success case
	cmd := &cobra.Command{}
	err = runExplorer(cmd, []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, tmpDir, capturedPath)

	// 2. Default path (.)
	// Since we are not changing CWD, . is current dir.
	// But let's create a scenario where we pass nothing.
	capturedPath = ""
	err = runExplorer(cmd, []string{})
	require.NoError(t, err)
	assert.Equal(t, ".", capturedPath)

	// 3. Invalid path
	err = runExplorer(cmd, []string{filepath.Join(tmpDir, "non-existent")})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path")

	// 4. File path (not dir)
	err = runExplorer(cmd, []string{filePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")

	// 5. StartExplorer failure
	startExplorerFunc = func(path string) error {
		return fmt.Errorf("mock error")
	}
	err = runExplorer(cmd, []string{tmpDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "explorer failed")
}
