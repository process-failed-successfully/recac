package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitorCommand_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "monitor" {
			found = true
			break
		}
	}
	assert.True(t, found, "monitor command should be registered on rootCmd")
}

func TestMonitorCommand_Structure(t *testing.T) {
	cmd := monitorCmd
	assert.Equal(t, "monitor", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
}
