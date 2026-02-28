package main

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestConfigGetCmd(t *testing.T) {
	// Setup viper with some test data
	viper.Set("test.string", "hello")
	viper.Set("test.int", 42)
	viper.Set("test.bool", true)
	viper.Set("secrets.api_key", "super-secret-key")
	defer func() {
		// Clean up
		viper.Set("test.string", nil)
		viper.Set("test.int", nil)
		viper.Set("test.bool", nil)
		viper.Set("secrets.api_key", nil)
	}()

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:           "get existing string",
			args:           []string{"test.string"},
			expectedOutput: "hello\n",
		},
		{
			name:           "get existing int",
			args:           []string{"test.int"},
			expectedOutput: "42\n",
		},
		{
			name:           "get existing bool",
			args:           []string{"test.bool"},
			expectedOutput: "true\n",
		},
		{
			name:          "get missing key",
			args:          []string{"test.missing"},
			expectedError: true,
		},
		{
			name:           "get sensitive key redacted",
			args:           []string{"secrets.api_key"},
			expectedOutput: "[REDACTED]\n",
		},
		{
			name:           "get sensitive key unredacted",
			args:           []string{"secrets.api_key", "--show-sensitive"},
			expectedOutput: "super-secret-key\n",
		},
		{
			name:          "missing argument",
			args:          []string{},
			expectedError: true,
		},
		{
			name:          "too many arguments",
			args:          []string{"test_key", "extra"},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the root command to allow testing via Cobra
			cmd := rootCmd

			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)

			var errBuf bytes.Buffer
			cmd.SetErr(&errBuf)

			// Add "config get" before our specific args
			args := append([]string{"config", "get"}, tt.args...)
			cmd.SetArgs(args)

			err := cmd.Execute()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, outBuf.String())
			}
		})
	}
}
