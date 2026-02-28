package main

import (
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
