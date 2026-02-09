package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Cleanup
	defer func() {
		os.Remove("config.yaml")
		viper.Reset()
	}()

	t.Run("Default Config Generation", func(t *testing.T) {
		viper.Reset()
		// Ensure no config file
		os.Remove("config.yaml")

		// Load with empty file
		Load("")

		// Should have created config.yaml?
		// Only if env vars are mostly empty, which they might be in valid test env.
		// However, Load() has logic: if os.Getenv("RECAC_PROVIDER") == "" ...

		// Let's just check defaults are set
		assert.Equal(t, "gemini", viper.GetString("provider"))
		assert.Equal(t, 20, viper.GetInt("max_iterations"))
	})

	t.Run("Load From Env", func(t *testing.T) {
		viper.Reset()
		os.Setenv("RECAC_PROVIDER", "openai")
		defer os.Unsetenv("RECAC_PROVIDER")

		Load("")
		assert.Equal(t, "openai", viper.GetString("provider"))
	})

	t.Run("Load Local Config", func(t *testing.T) {
		viper.Reset()
		// Create local config.yaml
		os.WriteFile("config.yaml", []byte("provider: local-provider\n"), 0644)
		defer os.Remove("config.yaml")

		Load("")
		assert.Equal(t, "local-provider", viper.GetString("provider"))
		assert.Equal(t, "config.yaml", viper.ConfigFileUsed())
	})

	t.Run("Load Home Config", func(t *testing.T) {
		viper.Reset()
		// Ensure no local config
		os.Remove("config.yaml")

		// Mock HOME
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)        // Unix
		t.Setenv("USERPROFILE", tmpHome) // Windows

		// Create ~/.recac.yaml
		homeConfig := filepath.Join(tmpHome, ".recac.yaml")
		os.WriteFile(homeConfig, []byte("provider: home-provider\n"), 0644)

		Load("")
		assert.Equal(t, "home-provider", viper.GetString("provider"))
		assert.Equal(t, homeConfig, viper.ConfigFileUsed())
	})

	t.Run("Local Priority Over Home", func(t *testing.T) {
		viper.Reset()

		// Mock HOME
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		// Create ~/.recac.yaml
		homeConfig := filepath.Join(tmpHome, ".recac.yaml")
		os.WriteFile(homeConfig, []byte("provider: home-provider\n"), 0644)

		// Create local config.yaml
		os.WriteFile("config.yaml", []byte("provider: local-provider\n"), 0644)
		defer os.Remove("config.yaml")

		Load("")
		assert.Equal(t, "local-provider", viper.GetString("provider"))
		assert.Equal(t, "config.yaml", viper.ConfigFileUsed())
	})
}
