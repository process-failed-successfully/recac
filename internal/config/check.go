package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Check verifies if the configuration file exists and can be loaded.
func Check() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("config file not found")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configFile)
	}
	return nil
}

// Fix attempts to create a default configuration if it's missing.
func Fix() error {
	// Simple fix: create default config if missing
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")
	return viper.SafeWriteConfig()
}
