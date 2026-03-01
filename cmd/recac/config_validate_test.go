package main

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidateCommand_Valid(t *testing.T) {
	// Reset viper to ensure a clean state
	viper.Reset()
	defer viper.Reset()

	// Set some valid configuration values
	viper.Set("timeout", 100)
	viper.Set("agent_timeout", 300)
	viper.Set("port", 8080)

	// Execute the command
	output, err := executeCommand(rootCmd, "config", "validate")
	require.NoError(t, err)

	// Check output
	assert.Contains(t, output, "Configuration is valid.")
}

func TestConfigValidateCommand_Invalid(t *testing.T) {
	// Reset viper to ensure a clean state
	viper.Reset()
	defer viper.Reset()

	// Set some invalid configuration values
	viper.Set("timeout", -10)
	viper.Set("port", 99999)

	// Execute the command directly via RunE to capture the error correctly
	// instead of using executeCommand which might swallow or handle it differently
	cmd := configValidateCmd

	// Prepare mock output buffer
	var outBuf strings.Builder
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Call RunE
	err := cmd.RunE(cmd, []string{})

	// Should return an error
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "configuration validation failed")
	assert.Contains(t, errMsg, "timeout must be positive")
	assert.Contains(t, errMsg, "port must be between 1 and 65535")
}
