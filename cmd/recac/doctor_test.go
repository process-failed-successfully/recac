package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoctorCmd_NoDocker_NoConfig(t *testing.T) {
	// We expect the command to fail because the test env has no Docker or config
	output, err := executeCommand(rootCmd, "doctor")
	require.Error(t, err, "doctor command should fail when issues are found")

	// Check for Docker error messages
	require.Contains(t, output, "Running doctor checks...")
	require.Contains(t, output, "Docker daemon is not reachable")

	// Check for Config error messages
	require.Contains(t, output, "Config file not found")

	// Check for the final summary
	require.Contains(t, output, "Some checks failed")
}
