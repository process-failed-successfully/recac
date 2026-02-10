package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctorCmd(t *testing.T) {
	// Create a new root command and capture its output
	cmd, out, _ := newRootCmd()
	cmd.SetOut(out)

	// Execute the "doctor" command
	cmd.SetArgs([]string{"doctor"})
	err := cmd.Execute()
	assert.NoError(t, err)

	// Check the output
	output := out.String()
	// The new output format starts with a newline and an icon
	assert.Contains(t, output, "RECAC Doctor Report", "Output should contain the doctor header")

	// Check for specific checks based on new format (e.g., "✅ Config", "✅ Docker")
	assert.Contains(t, output, "Config", "Output should contain a configuration check")
	assert.Contains(t, output, "Dependency: git", "Output should contain a git dependency check")
	assert.Contains(t, output, "Dependency: docker", "Output should contain a docker dependency check")
	assert.Contains(t, output, "Docker", "Output should contain a Docker check")

	// Check for the summary line
	assert.True(t, strings.Contains(output, "checks failed") || strings.Contains(output, "All systems operational"), "Output should contain a summary")
}
