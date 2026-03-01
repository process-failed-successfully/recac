package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigResetCmd_Force(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Pre-populate the configuration file
	initialConfig := map[string]interface{}{
		"agent": map[string]interface{}{
			"provider": "openai",
			"model":    "gpt-4",
		},
		"max_iterations": 42,
		"mock":           true,
	}
	data, err := yaml.Marshal(&initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	// Setup mock functions for this test run
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return configPath
	}
	defer func() {
		viperConfigFileUsed = originalViperConfigFileUsed
	}()

	// Ensure Viper knows about this file for its own operations
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Pre-condition: Check config actually loaded
	assert.Equal(t, 42, viper.GetInt("max_iterations"))

	// Execute with --force
	output, err := executeCommand(rootCmd, "config", "reset", "--force")
	require.NoError(t, err)

	assert.Contains(t, output, "Successfully reset configuration.")

	// File should exist but be effectively empty/reset
	require.FileExists(t, configPath)

	// Read back from file
	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	// Assert everything is empty
	assert.False(t, viper.IsSet("max_iterations"))
	assert.False(t, viper.IsSet("agent.provider"))
	assert.False(t, viper.IsSet("mock"))
}

func TestConfigResetCmd_Interactive_Yes(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Pre-populate the configuration file
	initialConfig := map[string]interface{}{
		"test_key": "test_value",
	}
	data, err := yaml.Marshal(&initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return configPath
	}
	defer func() {
		viperConfigFileUsed = originalViperConfigFileUsed
	}()

	// Simulate interactive input "y\n"
	cmd := rootCmd
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	inBuf := bytes.NewBufferString("y\n")
	cmd.SetIn(inBuf)

	cmd.SetArgs([]string{"config", "reset"})

	// Need to manually reset the force flag because it might be set from previous test
	configResetForce = false

	err = cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "WARNING: This will delete all your configuration settings")
	assert.Contains(t, output, "Successfully reset configuration.")

	// Check it was reset
	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)
	assert.False(t, viper.IsSet("test_key"))
}

func TestConfigResetCmd_Interactive_No(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Pre-populate the configuration file
	initialConfig := map[string]interface{}{
		"test_key": "test_value",
	}
	data, err := yaml.Marshal(&initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return configPath
	}
	defer func() {
		viperConfigFileUsed = originalViperConfigFileUsed
	}()

	// Simulate interactive input "n\n"
	cmd := rootCmd
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	inBuf := bytes.NewBufferString("n\n")
	cmd.SetIn(inBuf)

	cmd.SetArgs([]string{"config", "reset"})

	configResetForce = false

	err = cmd.Execute()
	require.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "WARNING: This will delete all your configuration settings")
	assert.Contains(t, output, "Reset cancelled.")

	// Check it was NOT reset
	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)
	assert.True(t, viper.IsSet("test_key"))
	assert.Equal(t, "test_value", viper.GetString("test_key"))
}
