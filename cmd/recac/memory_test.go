package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCmd(t *testing.T) {
	// Setup temp dir for test state
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalWd)

	t.Run("Add Memory", func(t *testing.T) {
		buf := new(bytes.Buffer)
		// Cobra commands might use a global output stream or inherit from root if not explicitly set properly on execution.
		// SetOut on the command itself should work if using RunE directly on that command.
		memoryAddCmd.SetOut(buf)
		memoryAddCmd.SetErr(buf)

		// We need to execute the RunE function directly or Execute the command.
		// memoryAddCmd is a sub-command.
		err := memoryAddCmd.RunE(memoryAddCmd, []string{"Remember this test"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Memory added")

		// Verify file content
		content, err := os.ReadFile(".agent_state.json")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Remember this test")
	})

	t.Run("List Memory", func(t *testing.T) {
		buf := new(bytes.Buffer)
		memoryListCmd.SetOut(buf)
		memoryListCmd.SetErr(buf)

		err := memoryListCmd.RunE(memoryListCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Remember this test")
	})

	t.Run("Clear Memory", func(t *testing.T) {
		buf := new(bytes.Buffer)
		memoryClearCmd.SetOut(buf)
		memoryClearCmd.SetErr(buf)

		err := memoryClearCmd.RunE(memoryClearCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Memory cleared")

		// Verify empty list
		buf.Reset()
		memoryListCmd.SetOut(buf)
		memoryListCmd.SetErr(buf)
		err = memoryListCmd.RunE(memoryListCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "No memory items found")
	})
}
