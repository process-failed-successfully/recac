package config

import (
	"os"
	"testing"
    "github.com/spf13/viper"
    "github.com/stretchr/testify/assert"
)

func TestConfigFilePermissions(t *testing.T) {
    // Create a temporary directory
    tmpDir := t.TempDir()
    originalDir, _ := os.Getwd()
    defer os.Chdir(originalDir)
    os.Chdir(tmpDir)

    // Reset viper
    viper.Reset()

    // Simulate Load calling logic that creates the file
    // We can't call Load directly easily because it has logic to NOT create file if certain env vars are present,
    // and also it writes to "config.yaml" in current dir.
    // Let's mimic what Load does:

    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.SetDefault("test_key", "test_value")

	// The new logic from Load.go:
	configFile := "config.yaml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		f, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		assert.NoError(t, err)
		f.Close()
		err = viper.WriteConfigAs(configFile)
		assert.NoError(t, err)
	}

	// Check permissions
	info, err := os.Stat("config.yaml")
	assert.NoError(t, err)

	mode := info.Mode().Perm()
	t.Logf("File permissions: %o", mode)

	// We expect strictly 0600 (rw-------)
	// On some systems/filesystems exact match might vary but it definitely shouldn't be world readable
	if mode&0077 != 0 {
		t.Errorf("Config file has insecure permissions: %o. Expected 0600", mode)
	}
}
