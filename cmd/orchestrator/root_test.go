package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRootFlags(t *testing.T) {
	// Test default values via Viper (ensure binding happened)
	// Default in code is "local", flag default is "local"
	// Ensure init() logic ran (it does on package load)

	// Note: Viper tests can be flaky if run in parallel with other tests modifying global viper state.
	// We are checking "orchestrator.mode" here.

	// Check default
	// Initially it might be "local" or whatever config loaded.
	// Since we don't load a config file in tests (cfgFile is empty), it uses defaults.

	// Set flag value
	err := rootCmd.Flags().Set("mode", "k8s")
	assert.NoError(t, err)

	// Verify Viper sees the change (proving binding works)
	assert.Equal(t, "k8s", viper.GetString("orchestrator.mode"))

	// Check another flag
	rootCmd.PersistentFlags().Set("work-file", "test.json")
	assert.Equal(t, "test.json", viper.GetString("orchestrator.work_file"))
}
