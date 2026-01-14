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
	assert.True(t, strings.HasPrefix(output, "RECAC Doctor"), "Output should start with the doctor header")
	assert.Contains(t, output, "Configuration:", "Output should contain a configuration check")
	assert.Contains(t, output, "Dependency:", "Output should contain a dependency check")
	assert.Contains(t, output, "Docker:", "Output should contain a Docker check")
}
