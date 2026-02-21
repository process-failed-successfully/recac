package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckConfig(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Config Found", func(t *testing.T) {
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte("provider: test"), 0644)
		require.NoError(t, err)

		viper.SetConfigFile(configFile)
		err = checkConfig()
		assert.NoError(t, err)
	})

	t.Run("Config Missing", func(t *testing.T) {
		missingFile := filepath.Join(tmpDir, "missing.yaml")
		viper.SetConfigFile(missingFile)

		err := checkConfig()
		assert.Error(t, err)
	})

	t.Run("Config Empty Path", func(t *testing.T) {
		viper.SetConfigFile("")
		err := checkConfig()
		assert.Error(t, err)
	})
}

func TestFixConfig(t *testing.T) {
	viper.Reset()
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "fixed_config.yaml")
	cfgFile = configFile // cfgFile is global variable used by fixConfig

	// fixConfig should create the file with defaults
	err := fixConfig()
	assert.NoError(t, err)

	assert.FileExists(t, configFile)

	// Read content back
	content, err := os.ReadFile(configFile)
	require.NoError(t, err)
	// Defaults are gemini
	assert.Contains(t, string(content), "provider: gemini")
}

func TestCheckGo(t *testing.T) {
	// Assume 'go' is in PATH in test environment
	err := checkGo()
	assert.NoError(t, err)
}
