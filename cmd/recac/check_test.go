package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestFixConfig(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// 1. Test checkConfig when missing
	err := checkConfig()
	if err == nil {
		t.Error("Expected error when config is missing")
	}

	// 2. Test fixConfig
	// We need to set config path/name for viper to know where to write?
	// fixConfig uses SafeWriteConfig which writes to paths.
	viper.AddConfigPath(tmpDir)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	err = fixConfig()
	if err != nil {
		t.Fatalf("fixConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "config.yaml")); os.IsNotExist(err) {
		t.Error("config.yaml was not created")
	}

	// 3. Test checkConfig after fix
	// We need to reload config for viper to know it's there?
	// Or checkConfig relies on viper.ConfigFileUsed().
	// SafeWriteConfig sets ConfigFileUsed? No.
	// We might need to call ReadInConfig.
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig failed: %v", err)
	}

	err = checkConfig()
	if err != nil {
		t.Errorf("checkConfig failed after fix: %v", err)
	}
}
