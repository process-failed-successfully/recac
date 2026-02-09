package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Cleanup any potentially leftover files
	defer func() {
		os.Remove("config.yaml")
		viper.Reset()
	}()

	t.Run("Default Config Generation in Home", func(t *testing.T) {
		viper.Reset()

		// Mock HOME
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Ensure no config file in current dir
		os.Remove("config.yaml")

		// Load with empty file
		Load("")

		// Should have created .recac.yaml in HOME
		expectedConfig := filepath.Join(tmpHome, ".recac.yaml")
		_, err := os.Stat(expectedConfig)
		assert.NoError(t, err, "Should create .recac.yaml in HOME")

		// Should NOT create config.yaml in current dir
		_, err = os.Stat("config.yaml")
		assert.True(t, os.IsNotExist(err), "Should NOT create config.yaml in current dir")
	})

	t.Run("Prioritize Local Config", func(t *testing.T) {
		viper.Reset()

		// Mock HOME
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Create local config.yaml
		os.WriteFile("config.yaml", []byte("provider: local_provider"), 0644)
		defer os.Remove("config.yaml")

		// Create home config (should be ignored)
		homeConfig := filepath.Join(tmpHome, ".recac.yaml")
		os.WriteFile(homeConfig, []byte("provider: home_provider"), 0644)

		Load("")

		assert.Equal(t, "local_provider", viper.GetString("provider"))
	})

	t.Run("Fallback to Home Config", func(t *testing.T) {
		viper.Reset()

		// Mock HOME
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		// Ensure no local config
		os.Remove("config.yaml")

		// Create home config
		homeConfig := filepath.Join(tmpHome, ".recac.yaml")
		os.WriteFile(homeConfig, []byte("provider: home_provider"), 0644)

		Load("")

		assert.Equal(t, "home_provider", viper.GetString("provider"))
	})

	t.Run("Load From Env", func(t *testing.T) {
		viper.Reset()

		// Mock HOME to avoid side effects
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		t.Setenv("RECAC_PROVIDER", "openai")
		// No need to unset, t.Setenv restores automatically

		Load("")
		assert.Equal(t, "openai", viper.GetString("provider"))
	})
}
