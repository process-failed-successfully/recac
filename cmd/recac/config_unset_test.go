package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnsetCommand(t *testing.T) {
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

	tests := []struct {
		name          string
		keyToUnset    string
		expectedExist map[string]bool
	}{
		{
			name:       "Unset top-level key",
			keyToUnset: "max_iterations",
			expectedExist: map[string]bool{
				"max_iterations": false,
				"agent.provider": true,
				"mock":           true,
			},
		},
		{
			name:       "Unset nested key",
			keyToUnset: "agent.provider",
			expectedExist: map[string]bool{
				"agent.provider": false,
				"agent.model":    true,
				"mock":           true,
			},
		},
		{
			name:       "Unset non-existent key",
			keyToUnset: "does.not.exist",
			expectedExist: map[string]bool{
				"agent.model": true,
				"mock":        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Rewrite the initial state for each test
			data, _ := yaml.Marshal(&initialConfig)
			os.WriteFile(configPath, data, 0644)

			// Setup mock functions for this test run
			originalViperConfigFileUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string {
				return configPath
			}
			defer func() {
				viperConfigFileUsed = originalViperConfigFileUsed
			}()

			output, err := executeCommand(rootCmd, "config", "unset", tt.keyToUnset)
			require.NoError(t, err)

			if tt.name == "Unset non-existent key" {
				assert.Contains(t, output, "not found")
			} else {
				assert.Contains(t, output, "Successfully unset "+tt.keyToUnset)
			}

			// Read back from file
			viper.Reset()
			viper.SetConfigFile(configPath)
			err = viper.ReadInConfig()
			require.NoError(t, err)

			for checkKey, expectedExists := range tt.expectedExist {
				if expectedExists {
					assert.True(t, viper.IsSet(checkKey), "Expected %s to exist", checkKey)
				} else {
					assert.False(t, viper.IsSet(checkKey), "Expected %s to NOT exist", checkKey)
				}
			}
		})
	}
}

func TestDeleteNestedKey(t *testing.T) {
	tests := []struct {
		name       string
		m          map[string]interface{}
		keys       []string
		expected   map[string]interface{}
		wasDeleted bool
	}{
		{
			name: "Top level key",
			m: map[string]interface{}{
				"a": 1,
				"b": 2,
			},
			keys: []string{"a"},
			expected: map[string]interface{}{
				"b": 2,
			},
			wasDeleted: true,
		},
		{
			name: "Nested key string map",
			m: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 2,
					"c": 3,
				},
			},
			keys: []string{"a", "b"},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"c": 3,
				},
			},
			wasDeleted: true,
		},
		{
			name: "Nested key interface map",
			m: map[string]interface{}{
				"a": map[interface{}]interface{}{
					"b": 2,
					"c": 3,
				},
			},
			keys: []string{"a", "b"},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"c": 3,
				},
			},
			wasDeleted: true,
		},
		{
			name: "Remove empty map after delete",
			m: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 2,
				},
				"c": 3,
			},
			keys: []string{"a", "b"},
			expected: map[string]interface{}{
				"c": 3,
			},
			wasDeleted: true,
		},
		{
			name: "Not found",
			m: map[string]interface{}{
				"a": 1,
			},
			keys:       []string{"b"},
			expected:   map[string]interface{}{"a": 1},
			wasDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleted := deleteNestedKey(tt.m, tt.keys)
			assert.Equal(t, tt.wasDeleted, deleted)
			assert.Equal(t, tt.expected, tt.m)
		})
	}
}
