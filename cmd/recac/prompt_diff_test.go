package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptDiffCmd(t *testing.T) {
	// Create temp dir for overrides
	tmpDir := t.TempDir()
	t.Setenv("RECAC_PROMPTS_DIR", tmpDir)

	// 1. Test Diff with No Override (Should be same)
	// We use "planner" prompt which we know exists
	output, err := executeCommand(rootCmd, "prompt", "diff", "planner")
	require.NoError(t, err)
	require.Contains(t, output, "No differences found")

	// 2. Create Override
	overridePath := filepath.Join(tmpDir, "planner.md")
	err = os.WriteFile(overridePath, []byte("This is an overridden planner prompt."), 0644)
	require.NoError(t, err)

	// 3. Test Diff with Override
	output, err = executeCommand(rootCmd, "prompt", "diff", "planner")
	require.NoError(t, err)

	// Should show diff
	require.Contains(t, output, "This is an overridden planner prompt.") // Active content
	require.Contains(t, output, "--- Embedded")
	require.Contains(t, output, "+++ Active")
	require.NotContains(t, output, "No differences found")
}
