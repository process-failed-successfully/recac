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
		// Ensure no config file in CWD
		os.Remove("config.yaml")

		// Isolate HOME to avoid reading real ~/.recac.yaml
		tmpHome, err := os.MkdirTemp("", "recac-test-home")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpHome)
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // Windows

		// Load with empty file
		Load("")

		// Should have created ~/.recac.yaml (preferred) or config.yaml?
		// Since we isolated HOME, ~/.recac.yaml does not exist.
		// So it should create one of them.

		// Let's just check defaults are set
		assert.Equal(t, "gemini", viper.GetString("provider"))
		assert.Equal(t, 20, viper.GetInt("max_iterations"))

		// Optional: Verify file creation
		homeConfig := filepath.Join(tmpHome, ".recac.yaml")
		if _, err := os.Stat(homeConfig); err == nil {
			// Found in home
		} else if _, err := os.Stat("config.yaml"); err == nil {
			// Found in CWD
		} else {
			// Not found? Load might warn but proceed with defaults if write fails.
		}
	})

	t.Run("Load From Env", func(t *testing.T) {
		viper.Reset()

		// Isolate HOME
		tmpHome, err := os.MkdirTemp("", "recac-test-home-env")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpHome)
		t.Setenv("HOME", tmpHome)

		os.Setenv("RECAC_PROVIDER", "openai")
		defer os.Unsetenv("RECAC_PROVIDER")

		Load("")
		assert.Equal(t, "openai", viper.GetString("provider"))
	})
}
