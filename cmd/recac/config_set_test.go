package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	tests := []struct {
		name          string
		key           string
		value         string
		expectedValue interface{}
	}{
		{
			name:          "Set string value",
			key:           "agent.provider",
			value:         "openai",
			expectedValue: "openai",
		},
		{
			name:          "Set boolean value true",
			key:           "mock",
			value:         "true",
			expectedValue: true,
		},
		{
			name:          "Set boolean value false",
			key:           "verbose",
			value:         "false",
			expectedValue: false,
		},
		{
			name:          "Set integer value",
			key:           "max_iterations",
			value:         "42",
			expectedValue: int(42),
		},
		{
			name:          "Set float value",
			key:           "temperature",
			value:         "0.7",
			expectedValue: float64(0.7),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.SetConfigFile(configPath)

			// Reset write func to write to our temp file
			originalViperSafeWriteConfig := viperSafeWriteConfig
			viperSafeWriteConfig = func() error {
				return viper.WriteConfigAs(configPath)
			}
			defer func() {
				viperSafeWriteConfig = originalViperSafeWriteConfig
			}()

			output, err := executeCommand(rootCmd, "config", "set", tt.key, tt.value)
			require.NoError(t, err)

			// Assert output message
			assert.Contains(t, output, "Successfully set "+tt.key+" to "+tt.value)

			// Assert viper state
			assert.Equal(t, tt.expectedValue, viper.Get(tt.key))

			// Assert file state
			viper.Reset()
			viper.SetConfigFile(configPath)
			err = viper.ReadInConfig()
			require.NoError(t, err, "Config file should exist and be readable")
			assert.Equal(t, tt.expectedValue, viper.Get(tt.key))
		})
	}
}

func TestSetConfigKey_EmptyConfigFile(t *testing.T) {
	// Setup mock viperConfigFileUsed
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return ""
	}
	defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

	// Setup mock viperSafeWriteConfig
	originalViperSafeWriteConfig := viperSafeWriteConfig
	viperSafeWriteConfig = func() error {
		return nil
	}
	defer func() { viperSafeWriteConfig = originalViperSafeWriteConfig }()

	cmd := setCmd
	err := setConfigKey(cmd, []string{"agent.provider", "openai"})
	require.NoError(t, err)
	assert.Equal(t, "openai", viper.Get("agent.provider"))
}

func TestSetConfigKey_WriteConfigAsFallback(t *testing.T) {
	// Setup mock viperConfigFileUsed
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return "test_config.yaml"
	}
	defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

	// Force viperSafeWriteConfig to fail so it falls back
	originalViperSafeWriteConfig := viperSafeWriteConfig
	viperSafeWriteConfig = func() error {
		return errors.New("safe write failed")
	}
	defer func() { viperSafeWriteConfig = originalViperSafeWriteConfig }()

	// Create a custom command to test the error fallback branch.
	// We can't easily mock viper.WriteConfigAs without writing to a file,
	// so we'll just let it fail to write to an invalid path or expect a success if it writes correctly to pwd.

	// We'll write to a temp file to test fallback success
	tempFile := t.TempDir() + "/test_fallback.yaml"
	viperConfigFileUsed = func() string {
		return tempFile
	}

	cmd := setCmd
	err := setConfigKey(cmd, []string{"fallback.key", "fallback_value"})
	require.NoError(t, err)
	assert.Equal(t, "fallback_value", viper.Get("fallback.key"))
}

func TestSetConfigKey_WriteConfigAsFailure(t *testing.T) {
	// Setup mock viperConfigFileUsed
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return "/invalid/path/that/cannot/be/written/config.yaml"
	}
	defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

	// Force viperSafeWriteConfig to fail so it falls back
	originalViperSafeWriteConfig := viperSafeWriteConfig
	viperSafeWriteConfig = func() error {
		return errors.New("safe write failed")
	}
	defer func() { viperSafeWriteConfig = originalViperSafeWriteConfig }()

	cmd := setCmd
	err := setConfigKey(cmd, []string{"fail.key", "fail_value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write config to")
}
