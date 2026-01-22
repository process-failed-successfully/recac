package main

import (
	"recac/internal/ui"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitorCmd_Structure(t *testing.T) {
	assert.Equal(t, "monitor", monitorCmd.Use)
	assert.NotNil(t, monitorCmd.RunE)
}

// MockStartMonitorDashboard overrides the UI starter for testing
var mockMonitorStarted bool
var mockMonitorCallbacks ui.ActionCallbacks

// To verify logic, we can inspect the run function's behavior regarding session manager creation.
// But since `RunE` creates everything inside, it's hard to test without running it.
// And running it starts a TUI which blocks.

// Alternative: We check if `monitorCmd` is registered in `rootCmd`.
func TestMonitorCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "monitor" {
			found = true
			break
		}
	}
	assert.True(t, found, "monitor command should be registered in rootCmd")
}
