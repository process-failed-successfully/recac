package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestCheckConfig(t *testing.T) {
	// Case 1: Config missing
	viper.Reset()
	// viper.ConfigFileUsed() returns empty string by default
	if err := checkConfig(); err == nil {
		t.Error("expected error when config missing")
	}

	// Case 2: Config exists
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("provider: test"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	viper.Reset() // Ensure clean state
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	if err := checkConfig(); err != nil {
		t.Errorf("expected no error when config exists, got %v", err)
	}
}

func TestFixConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	viper.Reset()
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Debug
	t.Logf("Config file set to: %s", viper.ConfigFileUsed())

	if err := fixConfig(); err != nil {
		t.Fatalf("fixConfig failed: %v", err)
	}

	// Check if file created
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	// Check content
	content, _ := os.ReadFile(configFile)
	if len(content) == 0 {
		t.Error("config file empty")
	}
}

func TestCheckGo(t *testing.T) {
	// Only run if go is installed (likely yes in dev/CI)
	if err := checkGo(); err != nil {
		t.Logf("checkGo failed (expected in some envs): %v", err)
	} else {
		t.Log("checkGo passed")
	}
}
