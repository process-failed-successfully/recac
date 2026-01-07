package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSettingsCmd(t *testing.T) {
	// Create a temporary directory for the config file
	tmpDir, err := os.MkdirTemp("", "recac-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a temporary config file
	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("key1: value1\nkey2: value2\n"), 0644)
	assert.NoError(t, err)

	// Set Viper to use the temporary config file
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	t.Run("get", func(t *testing.T) {
		t.Run("existing key", func(t *testing.T) {
			var out bytes.Buffer
			settingsGetCmd.SetOut(&out)

			err := settingsGetCmd.RunE(settingsGetCmd, []string{"key1"})
			assert.NoError(t, err)
			assert.Equal(t, "value1\n", out.String())
		})

		t.Run("non-existing key", func(t *testing.T) {
			err := settingsGetCmd.RunE(settingsGetCmd, []string{"nonexistent"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "key not found")
		})
	})

	t.Run("set", func(t *testing.T) {
		var out bytes.Buffer
		settingsSetCmd.SetOut(&out)

		err := settingsSetCmd.RunE(settingsSetCmd, []string{"key3", "value3"})
		assert.NoError(t, err)
		assert.Equal(t, "Set key3 = value3\n", out.String())

		// Verify the value was set in Viper
		assert.Equal(t, "value3", viper.GetString("key3"))

		// Verify the config file was updated
		content, err := os.ReadFile(configFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "key3: value3")
	})

	t.Run("view", func(t *testing.T) {
		var out bytes.Buffer
		settingsViewCmd.SetOut(&out)

		err := settingsViewCmd.RunE(settingsViewCmd, []string{})
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "key1: value1")
		assert.Contains(t, output, "key2: value2")
		assert.Contains(t, output, "key3: value3")
	})
}
