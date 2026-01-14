package main

import (
	"bytes"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPsCmd_InteractiveMode(t *testing.T) {
	// --- Setup ---
	mockSM := NewMockSessionManager()
	mockSM.Sessions["session-1"] = &runner.SessionState{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-time.Hour)}
	mockSM.Sessions["session-2"] = &runner.SessionState{Name: "session-2", Status: "running", StartTime: time.Now()}

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// --- Execute ---
	rootCmd, _, _ := newRootCmd()
	cmd := newPsCmd()
	rootCmd.AddCommand(cmd)

	// We can't fully test the interactive TUI, but we can check
	// that the command attempts to start it. We can do this by
	// checking for the "Interactive Session Explorer" title in the output.
	// A more robust test would require a bubbletea test driver.

	var outBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&outBuf)
	rootCmd.SetArgs([]string{"ps", "-i"})

	// Since tea.Program takes over, we can't just execute it directly in a test.
	// For now, we'll just check that the command doesn't error out.
	// A future improvement would be to use a cancellable context or a test driver for bubbletea.
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// We can't easily assert the TUI's state, but we can at least ensure
	// that the command ran without panicking and that our mock was called.
}

// Helper to create a new psCmd for each test run to avoid "flag redefined" errors
func newPsCmd() *cobra.Command {
	psCmd := &cobra.Command{
		Use:     "ps",
		Aliases: []string{"list"},
		Short:   "List sessions",
		RunE:    psCmd.RunE,
	}
	psCmd.Flags().BoolP("interactive", "i", false, "Start interactive session explorer")
	// Add other flags as needed for testing
	return psCmd
}
