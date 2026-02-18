package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	setupProvider string
	setupModel    string
	setupAPIKey   string
	setupJiraURL  string
	setupJiraUser string
	setupJiraToken string
	setupJiraLabel string
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up RECAC configuration",
	Long:  `Configure RECAC settings, including provider, model, API keys, and Jira integration.`,
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVar(&setupProvider, "provider", "", "AI Provider (gemini, openai, etc.)")
	setupCmd.Flags().StringVar(&setupModel, "model", "", "Model name")
	setupCmd.Flags().StringVar(&setupAPIKey, "api-key", "", "API Key for the provider")
	setupCmd.Flags().StringVar(&setupJiraURL, "jira-url", "", "Jira URL")
	setupCmd.Flags().StringVar(&setupJiraUser, "jira-user", "", "Jira Email/Username")
	setupCmd.Flags().StringVar(&setupJiraToken, "jira-token", "", "Jira API Token")
	setupCmd.Flags().StringVar(&setupJiraLabel, "jira-label", "recac-agent", "Jira Label for agents to watch")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if setupProvider == "" {
		return fmt.Errorf("--provider is required")
	}

	// Update Viper settings
	viper.Set("provider", setupProvider)
	if setupModel != "" {
		viper.Set("model", setupModel)
	}

	if setupJiraURL != "" {
		viper.Set("jira.url", setupJiraURL)
		viper.Set("jira.username", setupJiraUser)
		viper.Set("orchestrator.jira_label", setupJiraLabel)
	}

	// Determine Config File Path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			configFile = "config.yaml"
		} else {
			configFile = filepath.Join(home, ".recac.yaml")
		}
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		// If file doesn't exist, WriteConfigAs fails. Try SafeWriteConfigAs?
		// Actually WriteConfigAs might fail if dir doesn't exist.
		os.MkdirAll(filepath.Dir(configFile), 0755)
		if err := viper.WriteConfigAs(configFile); err != nil {
			// Try creating the file first if it doesn't exist
			if err := viper.SafeWriteConfigAs(configFile); err != nil {
				fmt.Printf("Warning: Could not write %s: %v\n", configFile, err)
			}
		}
	}
	fmt.Printf("Configuration saved to %s\n", configFile)

	// Write to .env
	var linesToAppend []string

	if setupAPIKey != "" {
		envKey := ""
		switch setupProvider {
		case "gemini":
			envKey = "GEMINI_API_KEY"
		case "openai":
			envKey = "OPENAI_API_KEY"
		case "anthropic":
			envKey = "ANTHROPIC_API_KEY"
		default:
			envKey = fmt.Sprintf("%s_API_KEY", strings.ToUpper(setupProvider))
		}
		linesToAppend = append(linesToAppend, fmt.Sprintf("%s=%s", envKey, setupAPIKey))
	}

	if setupJiraToken != "" {
		linesToAppend = append(linesToAppend, fmt.Sprintf("JIRA_API_TOKEN=%s", setupJiraToken))
	}

	if len(linesToAppend) > 0 {
		f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Printf("Error opening .env: %v\n", err)
		} else {
			defer f.Close()
			for _, line := range linesToAppend {
				f.WriteString(line + "\n")
			}
			fmt.Println("Secrets saved to .env")
		}
	}

	fmt.Println("Setup complete!")
	return nil
}
