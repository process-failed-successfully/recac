package main

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestGetCmd(t *testing.T) {
	// Set up viper for testing
	viper.Reset()
	defer viper.Reset()
	viper.Set("test_key", "test_value")
	viper.Set("test_int", 42)
	viper.Set("test_bool", true)

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  string
	}{
		{
			name:           "existing string key",
			args:           []string{"test_key"},
			expectedOutput: "test_value\n",
		},
		{
			name:           "existing integer key",
			args:           []string{"test_int"},
			expectedOutput: "42\n",
		},
		{
			name:           "existing boolean key",
			args:           []string{"test_bool"},
			expectedOutput: "true\n",
		},
		{
			name:          "missing key",
			args:          []string{"missing_key"},
			expectedError: "key not found: missing_key",
		},
		{
			name:          "missing argument",
			args:          []string{},
			expectedError: "accepts 1 arg(s), received 0",
		},
		{
			name:          "too many arguments",
			args:          []string{"test_key", "extra"},
			expectedError: "accepts 1 arg(s), received 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the command
			cmd := rootCmd

			// Capture output
			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)

			// Capture errors from cobra framework validation
			var errBuf bytes.Buffer
			cmd.SetErr(&errBuf)

			// Set args and execute
			args := append([]string{"config", "get"}, tt.args...)
			cmd.SetArgs(args)
			err := cmd.Execute()

			if tt.expectedError != "" {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, outBuf.String())
			}
		})
	}
}
