package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// GetDefaultConfig returns a map containing the default configuration values.
func GetDefaultConfig() map[string]interface{} {
	slackEnabled := false
	if os.Getenv("SLACK_BOT_USER_TOKEN") != "" {
		slackEnabled = true
	}

	defaults := map[string]interface{}{
		"provider":                                         "gemini",
		"model":                                            "gemini-pro",
		"max_iterations":                                   20,
		"manager_frequency":                                5,
		"timeout":                                          300,
		"docker_timeout":                                   600,
		"bash_timeout":                                     600,
		"agent_timeout":                                    300,
		"metrics_port":                                     2112,
		"verbose":                                          false,
		"git_user_email":                                   "recac-agent@example.com",
		"git_user_name":                                    "RECAC Agent",
		"notifications.slack.enabled":                      slackEnabled,
		"notifications.slack.channel":                      "#general",
		"notifications.slack.events.on_start":              true,
		"notifications.slack.events.on_success":            true,
		"notifications.slack.events.on_failure":            true,
		"notifications.slack.events.on_user_interaction":   true,
		"notifications.slack.events.on_project_complete":   true,
	}

	// Check for standard JIRA_URL if RECAC_JIRA_URL is not set
	if os.Getenv("RECAC_JIRA_URL") == "" && os.Getenv("JIRA_URL") != "" {
		defaults["jira.url"] = os.Getenv("JIRA_URL")
	}

	return defaults
}

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
		// Search config in home directory with name ".recac" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("RECAC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// Set defaults
	defaults := GetDefaultConfig()
	for k, v := range defaults {
		viper.SetDefault(k, v)
	}

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
