package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load initializes the configuration from file and environment variables.
func Load(cfgFile string) {
	// explicit .env loading
	if err := godotenv.Load(); err != nil {
		// handle error if you want, or ignore if .env is missing
		// fmt.Println("No .env file found")
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".recac" (without extension).
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

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		// Config file not found; create one with defaults ONLY if not in agent/orchestrator mode
		// We avoid creating file if we are running in specialized modes often, but adhering to existing logic:
		if os.Getenv("RECAC_PROVIDER") == "" && os.Getenv("RECAC_AGENT_PROVIDER") == "" && os.Getenv("RECAC_ORCHESTRATOR_MODE") == "" {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok || true {
				// check if we already tried to read a specific file
				if cfgFile == "" {
					// Write config to current directory
					viper.SetConfigName("config")
					viper.SetConfigType("yaml")
					viper.AddConfigPath(".")

					// Attempt to write
					// Note: Existing logic swallowed errors partially or just printed warnings.
					// We will be slightly safer.
					if err := viper.SafeWriteConfig(); err != nil {
						// Ignore if already exists (SafeWriteConfig error)
						// But if it doesn't exist and failed, we might warn.
						// Checking existence first is better as per original code
						if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
							if err := viper.WriteConfigAs("config.yaml"); err != nil {
								fmt.Fprintf(os.Stderr, "Warning: Failed to create default config file: %v\n", err)
							} else {
								fmt.Println("Created default configuration file: config.yaml")
							}
						}
					} else {
						fmt.Println("Created default configuration file: config.yaml")
					}
				}
			}
		}
	}
}
