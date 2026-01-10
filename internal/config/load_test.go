package config

import (
	"os"
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
}
