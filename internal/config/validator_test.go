package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func()
		wantError bool
		errMsg    string
	}{
		{
			name: "Valid Configuration",
			setup: func() {
				viper.Set("timeout", "30s")
				viper.Set("max_iterations", 10)
				viper.Set("workers", 5)
				viper.Set("port", 8080)
			},
			wantError: false,
		},
		{
			name: "Invalid Timeout (Negative Duration)",
			setup: func() {
				viper.Set("timeout", -10*time.Second)
			},
			wantError: true,
			errMsg:    "timeout must be positive",
		},
		{
			name: "Invalid Timeout (Negative Int)",
			setup: func() {
				viper.Set("timeout", -10)
			},
			wantError: true,
			errMsg:    "timeout must be positive",
		},
		{
			name: "Invalid Max Iterations",
			setup: func() {
				viper.Set("max_iterations", 0)
			},
			wantError: true,
			errMsg:    "max_iterations must be positive",
		},
		{
			name: "Invalid Workers",
			setup: func() {
				viper.Set("workers", -1)
			},
			wantError: true,
			errMsg:    "workers must be positive",
		},
		{
			name: "Invalid Port (Too Low)",
			setup: func() {
				viper.Set("port", 0)
			},
			wantError: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name: "Invalid Port (Too High)",
			setup: func() {
				viper.Set("port", 70000)
			},
			wantError: true,
			errMsg:    "port must be between 1 and 65535",
		},
		{
			name: "Multiple Errors",
			setup: func() {
				viper.Set("timeout", -5)
				viper.Set("port", 80000)
			},
			wantError: true,
			errMsg:    "configuration validation failed",
		},
		{
			name: "Invalid Agent Timeout",
			setup: func() {
				viper.Set("agent_timeout", -1)
			},
			wantError: true,
			errMsg:    "agent_timeout must be positive",
		},
		{
			name: "Invalid Docker Timeout",
			setup: func() {
				viper.Set("docker_timeout", -1)
			},
			wantError: true,
			errMsg:    "docker_timeout must be positive",
		},
		{
			name: "Invalid Bash Timeout",
			setup: func() {
				viper.Set("bash_timeout", -1)
			},
			wantError: true,
			errMsg:    "bash_timeout must be positive",
		},
		{
			name: "Invalid Max Agents",
			setup: func() {
				viper.Set("max_agents", -1)
			},
			wantError: true,
			errMsg:    "max_agents must be positive",
		},
		{
			name: "Invalid Metrics Port",
			setup: func() {
				viper.Set("metrics_port", 99999)
			},
			wantError: true,
			errMsg:    "metrics_port must be between 1 and 65535",
		},
		{
			name: "Invalid Manager Frequency",
			setup: func() {
				viper.Set("manager_frequency", 0)
			},
			wantError: true,
			errMsg:    "manager_frequency must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()
			
			// Run setup
			if tt.setup != nil {
				tt.setup()
			}

			err := ValidateConfig()
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateConfig() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateConfig() unexpected error: %v", err)
				}
			}
		})
	}
}
