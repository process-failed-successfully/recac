package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// ValidateConfig validates configuration values and returns an error if any are invalid.
// This function should be called after viper has loaded the configuration.
func ValidateConfig() error {
	var errors []string

	// Validate timeout values (must be positive)
	// Try GetDuration first, then fall back to GetInt (seconds) if that fails
	if viper.IsSet("timeout") {
		var timeout time.Duration
		if d := viper.GetDuration("timeout"); d != 0 {
			timeout = d
		} else if s := viper.GetInt("timeout"); s != 0 {
			timeout = time.Duration(s) * time.Second
		}
		if timeout <= 0 {
			errors = append(errors, fmt.Sprintf("timeout must be positive, got: %v", timeout))
		}
	}

	// Validate agent timeout (if set)
	if viper.IsSet("agent_timeout") {
		var timeout time.Duration
		if d := viper.GetDuration("agent_timeout"); d != 0 {
			timeout = d
		} else if s := viper.GetInt("agent_timeout"); s != 0 {
			timeout = time.Duration(s) * time.Second
		}
		if timeout <= 0 {
			errors = append(errors, fmt.Sprintf("agent_timeout must be positive, got: %v", timeout))
		}
	}

	// Validate docker timeout (if set)
	if viper.IsSet("docker_timeout") {
		var timeout time.Duration
		if d := viper.GetDuration("docker_timeout"); d != 0 {
			timeout = d
		} else if s := viper.GetInt("docker_timeout"); s != 0 {
			timeout = time.Duration(s) * time.Second
		}
		if timeout <= 0 {
			errors = append(errors, fmt.Sprintf("docker_timeout must be positive, got: %v", timeout))
		}
	}

	// Validate bash timeout (if set)
	if viper.IsSet("bash_timeout") {
		var timeout time.Duration
		if d := viper.GetDuration("bash_timeout"); d != 0 {
			timeout = d
		} else if s := viper.GetInt("bash_timeout"); s != 0 {
			timeout = time.Duration(s) * time.Second
		}
		if timeout <= 0 {
			errors = append(errors, fmt.Sprintf("bash_timeout must be positive, got: %v", timeout))
		}
	}

	// Validate max_iterations (if set, must be positive)
	if viper.IsSet("max_iterations") {
		maxIter := viper.GetInt("max_iterations")
		if maxIter <= 0 {
			errors = append(errors, fmt.Sprintf("max_iterations must be positive, got: %d", maxIter))
		}
	}

	// Validate max_agents (if set, must be positive)
	if viper.IsSet("max_agents") {
		maxAgents := viper.GetInt("max_agents")
		if maxAgents <= 0 {
			errors = append(errors, fmt.Sprintf("max_agents must be positive, got: %d", maxAgents))
		}
	}

	// Validate workers (if set, must be positive)
	if viper.IsSet("workers") {
		workers := viper.GetInt("workers")
		if workers <= 0 {
			errors = append(errors, fmt.Sprintf("workers must be positive, got: %d", workers))
		}
	}

	// Validate port numbers (if set, must be in valid range 1-65535)
	if viper.IsSet("port") {
		port := viper.GetInt("port")
		if port < 1 || port > 65535 {
			errors = append(errors, fmt.Sprintf("port must be between 1 and 65535, got: %d", port))
		}
	}

	// Validate metrics_port (if set)
	if viper.IsSet("metrics_port") {
		port := viper.GetInt("metrics_port")
		if port < 1 || port > 65535 {
			errors = append(errors, fmt.Sprintf("metrics_port must be between 1 and 65535, got: %d", port))
		}
	}

	// Validate manager_frequency (if set, must be positive)
	if viper.IsSet("manager_frequency") {
		freq := viper.GetInt("manager_frequency")
		if freq <= 0 {
			errors = append(errors, fmt.Sprintf("manager_frequency must be positive, got: %d", freq))
		}
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

// ValidateAndExit validates the configuration and exits with a non-zero code if validation fails.
// This is a convenience function that prints errors to stderr and exits.
func ValidateAndExit() {
	if err := ValidateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
