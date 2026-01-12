package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopCommand(t *testing.T) {
	// Test that the command is correctly registered with the root command.
	rootCmd, _, _ := newRootCmd()
	topCmdFound, _, err := rootCmd.Find([]string{"top"})
	require.NoError(t, err, "The 'top' command should be registered.")
	require.NotNil(t, topCmdFound, "The 'top' command should not be nil.")
	assert.Equal(t, "top", topCmdFound.Name(), "The command name should be 'top'.")

	// Test the initialization of the TUI model.
	mockSm := NewMockSessionManager()
	model := newTopModel(mockSm)

	require.NotNil(t, model, "The TUI model should not be nil.")
	assert.Equal(t, mockSm, model.sm, "The model should have the correct session manager.")
	assert.NotNil(t, model.table, "The model's table should not be nil.")

	// Verify that the table columns are set up as expected.
	columns := model.table.Columns()
	expectedColumnTitles := []string{"NAME", "STATUS", "STARTED", "DURATION", "PROMPT", "COMPLETION", "TOTAL", "COST"}
	require.Len(t, columns, len(expectedColumnTitles), "The number of columns should match the expected count.")

	for i, col := range columns {
		assert.Equal(t, expectedColumnTitles[i], col.Title, "Column titles should be correctly set.")
	}
}
