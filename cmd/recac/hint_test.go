package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestHintCommand(t *testing.T) {
	// Create a buffer to capture the command's output
	var buf bytes.Buffer

	// Create a new instance of the root command
	rootCmd := &cobra.Command{Use: "recac"}
	initHintCmd(rootCmd)

	// Redirect the command's output to our buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"hint"})

	// Execute the command
	err := rootCmd.Execute()
	assert.NoError(t, err, "Hint command should execute without errors")

	// Read the output from the buffer
	output, err := io.ReadAll(&buf)
	assert.NoError(t, err, "Should be able to read the captured output")

	// Verify the output contains the expected cheatsheet text
	outputStr := string(output)
	assert.True(t, strings.Contains(outputStr, "recac CLI Cheatsheet"), "Output should contain the cheatsheet title")
	assert.True(t, strings.Contains(outputStr, "Session Management"), "Output should contain the Session Management section")
	assert.True(t, strings.Contains(outputStr, "recac start:"), "Output should contain the 'start' command")
	assert.True(t, strings.Contains(outputStr, "recac ls:"), "Output should contain the 'ls' command")
	assert.True(t, strings.Contains(outputStr, "recac ps:"), "Output should contain the 'ps' command")
}
