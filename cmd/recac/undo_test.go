package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUndoCommand_Integration(t *testing.T) {
	// Setup temp dir as workspace
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Mock writeFileFunc to actually write (default behavior)
	writeFileFunc = os.WriteFile

	// 1. Create initial file
	filePath := "test.txt"
	err := os.WriteFile(filePath, []byte("initial"), 0644)
	require.NoError(t, err)

	// 2. Perform safe write (Simulate modification)
	// This should trigger backup
	err = safeWriteFile(filePath, []byte("modified"), 0644)
	require.NoError(t, err)

	// Verify modification
	content, _ := os.ReadFile(filePath)
	assert.Equal(t, "modified", string(content))

	// 3. Run 'recac undo'
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// We need to construct a new undoCmd instance or reset global state if any?
	// undoCmd uses os.Getwd() internally, which we mocked via chdir.
	err = runUndo(cmd, []string{}) // Undo last
	require.NoError(t, err)

	// 4. Verify Restoration
	content, _ = os.ReadFile(filePath)
	assert.Equal(t, "initial", string(content))
	assert.Contains(t, buf.String(), "Undo successful")
}

func TestUndoListCommand_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// 1. Create file and modify safely
	os.WriteFile("a.txt", []byte("a"), 0644)
	safeWriteFile("a.txt", []byte("b"), 0644)

	// 2. Run list
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runUndoList(cmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "a.txt")
}
