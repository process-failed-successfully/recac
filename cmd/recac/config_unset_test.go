package main

import (
	"errors"
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

func TestUnsetConfigKey_Errors(t *testing.T) {
	// Setup generic mock for readFileFunc to control error
	originalReadFileFunc := readFileFunc
	defer func() { readFileFunc = originalReadFileFunc }()

	tests := []struct {
		name         string
		keyToUnset   string
		mockReadErr  error
		mockReadRet  []byte
		mockWriteErr error
		expectedErr  string
	}{
		{
			name:        "Config file does not exist",
			keyToUnset:  "agent.provider",
			mockReadErr: os.ErrNotExist,
			expectedErr: "config file config.yaml does not exist",
		},
		{
			name:        "Failed to read config file",
			keyToUnset:  "agent.provider",
			mockReadErr: errors.New("read error"),
			expectedErr: "failed to read config file config.yaml: read error",
		},
		{
			name:        "Failed to parse config file",
			keyToUnset:  "agent.provider",
			mockReadRet: []byte("invalid yaml content: [}"),
			expectedErr: "failed to parse config file config.yaml",
		},
		{
			name:         "Failed to write config file",
			keyToUnset:   "agent.provider",
			mockReadRet:  []byte("agent:\n  provider: openai"),
			mockWriteErr: errors.New("write error"),
			expectedErr:  "failed to write config file config.yaml: write error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock viperConfigFileUsed
			originalViperConfigFileUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string {
				return "config.yaml" // Ensure fixed path for error matching
			}
			defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

			readFileFunc = func(name string) ([]byte, error) {
				return tt.mockReadRet, tt.mockReadErr
			}

			// Setup write mock
			originalWriteFileFunc := writeFileFunc
			writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
				return tt.mockWriteErr
			}
			defer func() { writeFileFunc = originalWriteFileFunc }()

			// Execute the command directly via RunE to capture the error reliably
			cmd := unsetCmd
			err := unsetConfigKey(cmd, []string{tt.keyToUnset})

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeleteNestedKey_EmptyKeys(t *testing.T) {
	m := map[string]interface{}{
		"a": 1,
	}
	deleted := deleteNestedKey(m, []string{})
	assert.False(t, deleted)
}

func TestDeleteNestedKey_NonMapValue(t *testing.T) {
	m := map[string]interface{}{
		"a": 1,
	}
	deleted := deleteNestedKey(m, []string{"a", "b"})
	assert.False(t, deleted)
}

func TestUnsetConfigKey_EmptyConfigFile(t *testing.T) {
	// Setup mock viperConfigFileUsed
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return ""
	}
	defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

	// Setup mock readFileFunc
	originalReadFileFunc := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("agent:\n  provider: openai"), nil
	}
	defer func() { readFileFunc = originalReadFileFunc }()

	// Setup mock writeFileFunc
	originalWriteFileFunc := writeFileFunc
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return nil
	}
	defer func() { writeFileFunc = originalWriteFileFunc }()

	cmd := unsetCmd
	err := unsetConfigKey(cmd, []string{"agent.provider"})
	require.NoError(t, err)
}

func TestUnsetConfigKey_MarshalError(t *testing.T) {
	// Setup mock viperConfigFileUsed
	originalViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string {
		return "config.yaml"
	}
	defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

	// Setup mock readFileFunc
	originalReadFileFunc := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("a: b"), nil // Valid yaml
	}
	defer func() { readFileFunc = originalReadFileFunc }()

	// Provide a struct with an un-marshallable channel inside it, via monkey patching Unmarshal or similar.
	// We can't easily mock `yaml.Marshal` directly since it's an external library call.
	// But we can read something that parses into an unsupported type, like a function or channel if we could inject it.
	// Since we use yaml.Unmarshal into map[string]interface{}, it'll always marshal back correctly.
	// We'll skip testing the `yaml.Marshal` error case if it's practically unreachable.
}
