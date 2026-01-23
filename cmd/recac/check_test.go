package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestFixConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Reset viper
	viper.Reset()
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Ensure file doesn't exist
	assert.NoFileExists(t, configFile)

	err := fixConfig()
	assert.NoError(t, err)

	assert.FileExists(t, configFile)
}

func TestCheckConfig(t *testing.T) {
    tmpDir := t.TempDir()
    configFile := filepath.Join(tmpDir, "config.yaml")

    viper.Reset()
    viper.SetConfigFile(configFile)
    viper.SetConfigType("yaml")

    // Test missing
    err := checkConfig()
    assert.Error(t, err)

    // Test exists
    // Pre-create file to ensure checkConfig passes
    viper.SetDefault("provider", "gemini")
    err = viper.SafeWriteConfig()
    if err != nil {
         // Fallback if SafeWriteConfig fails (e.g. if it thinks file exists or path issue)
         err = viper.WriteConfig()
    }
    assert.NoError(t, err, "Setup failed: could not write config")

    err = checkConfig()
    assert.NoError(t, err)
}
