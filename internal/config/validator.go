package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

func validateTimeout(key string) string {
	if !viper.IsSet(key) {
		return ""
	}
	var timeout time.Duration
	if d := viper.GetDuration(key); d != 0 {
		timeout = d
	} else if s := viper.GetInt(key); s != 0 {
		timeout = time.Duration(s) * time.Second
	}
	if timeout <= 0 {
		return fmt.Sprintf("%s must be positive, got: %v", key, timeout)
	}
	return ""
}

func validatePositiveInt(key string) string {
	if !viper.IsSet(key) {
		return ""
	}
	val := viper.GetInt(key)
	if val <= 0 {
		return fmt.Sprintf("%s must be positive, got: %d", key, val)
	}
	return ""
}

func validatePort(key string) string {
	if !viper.IsSet(key) {
		return ""
	}
	port := viper.GetInt(key)
	if port < 1 || port > 65535 {
		return fmt.Sprintf("%s must be between 1 and 65535, got: %d", key, port)
	}
	return ""
}

// ValidateConfig validates configuration values and returns an error if any are invalid.
// This function should be called after viper has loaded the configuration.
func ValidateConfig() error {
	var errors []string

	if err := validateTimeout("timeout"); err != "" {
		errors = append(errors, err)
	}
	if err := validateTimeout("agent_timeout"); err != "" {
		errors = append(errors, err)
	}
	if err := validateTimeout("docker_timeout"); err != "" {
		errors = append(errors, err)
	}
	if err := validateTimeout("bash_timeout"); err != "" {
		errors = append(errors, err)
	}

	if err := validatePositiveInt("max_iterations"); err != "" {
		errors = append(errors, err)
	}
	if err := validatePositiveInt("max_agents"); err != "" {
		errors = append(errors, err)
	}
	if err := validatePositiveInt("workers"); err != "" {
		errors = append(errors, err)
	}
	if err := validatePositiveInt("manager_frequency"); err != "" {
		errors = append(errors, err)
	}

	if err := validatePort("port"); err != "" {
		errors = append(errors, err)
	}
	if err := validatePort("metrics_port"); err != "" {
		errors = append(errors, err)
	}

	// If there are any errors, return them
	if len(errors) > 0 {
		errorMsg := errors[0]
		for i := 1; i < len(errors); i++ {
			errorMsg += "\n  " + errors[i]
		}
		return fmt.Errorf("configuration validation failed:\n  %s", errorMsg)
	}

	return nil
}

// osExit is used to allow mocking os.Exit in tests
var osExit = os.Exit

// ValidateAndExit validates the configuration and exits with a non-zero code if validation fails.
// This is a convenience function that prints errors to stderr and exits.
func ValidateAndExit() {
	if err := ValidateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		osExit(1)
	}
}
