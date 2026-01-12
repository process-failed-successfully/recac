package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCheck returns a check function that can be controlled for testing.
func mockCheck(shouldPass bool, err error) func() (bool, error) {
	return func() (bool, error) {
		return shouldPass, err
	}
}

func TestRunChecks(t *testing.T) {
	// Keep original checks
	originalChecks := checks

	// Restore original checks after the test
	t.Cleanup(func() {
		checks = originalChecks
	})

	testCases := []struct {
		name          string
		setupChecks   func()
		expectedOut   string
		expectedErr   string
		shouldFail    bool
	}{
		{
			name: "All checks pass",
			setupChecks: func() {
				checks = []check{
					{Name: "Check 1", Run: mockCheck(true, nil)},
					{Name: "Check 2", Run: mockCheck(true, nil)},
				}
			},
			expectedOut: "✅ All checks passed!",
			shouldFail:  false,
		},
		{
			name: "One check fails",
			setupChecks: func() {
				checks = []check{
					{Name: "Check 1", Run: mockCheck(true, nil)},
					{Name: "Check 2", Run: mockCheck(false, nil)},
				}
			},
			expectedErr: "❌ Some checks failed.",
			shouldFail:    true,
		},
		{
			name: "One check errors",
			setupChecks: func() {
				checks = []check{
					{Name: "Check 1", Run: mockCheck(true, nil)},
					{Name: "Check 2", Run: mockCheck(false, fmt.Errorf("it broke")), Remedy: "Fix it."},
				}
			},
			expectedErr: "Error: it broke",
			shouldFail:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupChecks()

			// Redirect stdout and stderr
			oldOut, oldErr := os.Stdout, os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout, os.Stderr = wOut, wErr

			err := runChecks()

			// Close the writers and restore
			wOut.Close()
			wErr.Close()
			os.Stdout, os.Stderr = oldOut, oldErr

			// Read the output
			outBytes, _ := io.ReadAll(rOut)
			errBytes, _ := io.ReadAll(rErr)
			output := string(outBytes)
			errorOutput := string(errBytes)
			fullOutput := output + errorOutput

			if tc.shouldFail {
				require.Error(t, err)
				assert.Contains(t, fullOutput, tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Contains(t, fullOutput, tc.expectedOut)
			}
		})
	}
}

func TestCheckConfig(t *testing.T) {
	t.Run("Config file used", func(t *testing.T) {
		// Create a temporary config file
		tmpFile, err := os.CreateTemp("", "config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		viper.SetConfigFile(tmpFile.Name())
		viper.ReadInConfig()

		passed, err := checkConfig()
		assert.NoError(t, err)
		assert.True(t, passed)
	})

	t.Run("No config file", func(t *testing.T) {
		// Reset viper
		viper.Reset()
		// Ensure we're not reading any actual config
		viper.SetConfigFile("/tmp/non-existent-file-for-test.yaml")
		viper.ReadInConfig()

		var stdout bytes.Buffer
		os.Stdout = &stdout

		passed, err := checkConfig()

		os.Stdout = os.NewFile(uintptr(syscall.Stdout), "/dev/stdout")

		assert.Error(t, err)
		assert.False(t, passed)
		assert.Contains(t, err.Error(), "no config file found")
	})
}
func TestCheckAPIKey(t *testing.T) {
	t.Run("API key and provider set", func(t *testing.T) {
		viper.Set("agent_provider", "test-provider")
		viper.Set("api_key", "test-key")
		defer viper.Reset()

		passed, err := checkAPIKey()
		assert.NoError(t, err)
		assert.True(t, passed)
	})

	t.Run("Provider not set", func(t *testing.T) {
		viper.Set("api_key", "test-key")
		defer viper.Reset()

		passed, err := checkAPIKey()
		assert.Error(t, err)
		assert.False(t, passed)
	})

	t.Run("API key not set", func(t *testing.T) {
		viper.Set("agent_provider", "test-provider")
		defer viper.Reset()

		passed, err := checkAPIKey()
		assert.Error(t, err)
		assert.False(t, passed)
	})
}
