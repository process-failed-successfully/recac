package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigureCommand_Structure(t *testing.T) {
	assert.NotNil(t, configureCmd)
	assert.Equal(t, "configure", configureCmd.Use)
	assert.Equal(t, "Interactive configuration wizard", configureCmd.Short)

	// Ensure it's added to root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "configure" {
			found = true
			break
		}
	}
	assert.True(t, found, "configure command should be added to rootCmd")
}
