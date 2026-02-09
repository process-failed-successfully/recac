package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load initializes the configuration from file and environment variables.
func Load(cfgFile string) {
	// explicit .env loading
	if err := godotenv.Load(); err != nil {
		// handle error if you want, or ignore if .env is missing
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory with name "config" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("RECAC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// Check for standard JIRA_URL if RECAC_JIRA_URL is not set
	if os.Getenv("RECAC_JIRA_URL") == "" && os.Getenv("JIRA_URL") != "" {
		viper.SetDefault("jira.url", os.Getenv("JIRA_URL"))
	}

	// Set defaults
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")
	viper.SetDefault("max_iterations", 20)
	viper.SetDefault("manager_frequency", 5)
	viper.SetDefault("timeout", 300)
	viper.SetDefault("docker_timeout", 600)
	viper.SetDefault("bash_timeout", 600)
	viper.SetDefault("agent_timeout", 300)
	viper.SetDefault("metrics_port", 2112)
	viper.SetDefault("verbose", false)
	viper.SetDefault("git_user_email", "recac-agent@example.com")
	viper.SetDefault("git_user_name", "RECAC Agent")

	// Notification Defaults
	slackEnabled := false
	if os.Getenv("SLACK_BOT_USER_TOKEN") != "" {
		slackEnabled = true
	}
	viper.SetDefault("notifications.slack.enabled", slackEnabled)
	viper.SetDefault("notifications.slack.channel", "#general")
	viper.SetDefault("notifications.slack.events.on_start", true)
	viper.SetDefault("notifications.slack.events.on_success", true)
	viper.SetDefault("notifications.slack.events.on_failure", true)
	viper.SetDefault("notifications.slack.events.on_user_interaction", true)
	viper.SetDefault("notifications.slack.events.on_project_complete", true)

	// Attempt to read config
	err := viper.ReadInConfig()

	// If failed and cfgFile was not specified, try home directory
	if err != nil && cfgFile == "" {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			home, hErr := os.UserHomeDir()
			if hErr == nil {
				homeConfig := filepath.Join(home, ".recac.yaml")
				viper.SetConfigFile(homeConfig)
				err = viper.ReadInConfig()
			}
		}
	}

	if err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		// Config file not found; create one with defaults ONLY if not in agent/orchestrator mode
		if os.Getenv("RECAC_PROVIDER") == "" && os.Getenv("RECAC_AGENT_PROVIDER") == "" && os.Getenv("RECAC_ORCHESTRATOR_MODE") == "" {
			// We only create if cfgFile was NOT specified
			if cfgFile == "" {
				// Decide where to create default config
				targetFile := "config.yaml" // Default to current directory if home unavailable
				home, hErr := os.UserHomeDir()
				if hErr == nil {
					targetFile = filepath.Join(home, ".recac.yaml")
				}

				// Set config file to target
				viper.SetConfigFile(targetFile)
				viper.SetConfigType("yaml") // explicit type

				// Check if file exists (SafeWriteConfig logic)
				if _, statErr := os.Stat(targetFile); os.IsNotExist(statErr) {
					if wErr := viper.WriteConfigAs(targetFile); wErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to create default config file at %s: %v\n", targetFile, wErr)
					} else {
						fmt.Printf("Created default configuration file: %s\n", targetFile)
					}
				}
			}
		}
	}
}
