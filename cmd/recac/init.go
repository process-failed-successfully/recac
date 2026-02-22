package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	// Mocks for testing
	readPasswordFunc = term.ReadPassword
	stdinFd          = int(syscall.Stdin)
)

var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"setup"},
	Short:   "Initialize or update RECAC configuration",
	Long:    `Interactively setup the configuration for RECAC. This command will walk you through setting up your AI provider, API keys, and optional Jira integration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(input io.Reader, output io.Writer) error {
	fmt.Fprintln(output, "Welcome to RECAC Setup!")
	fmt.Fprintln(output, "-----------------------")
	fmt.Fprintln(output, "This wizard will help you configure your environment.")
	fmt.Fprintln(output, "")

	scanner := bufio.NewScanner(input)

	// Helper to read line
	readLine := func(prompt, defaultValue string) string {
		if defaultValue != "" {
			fmt.Fprintf(output, "%s [%s]: ", prompt, defaultValue)
		} else {
			fmt.Fprintf(output, "%s: ", prompt)
		}

		if !scanner.Scan() {
			return defaultValue
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return defaultValue
		}
		return line
	}

	// Helper to read password
	readSecret := func(prompt, defaultValue string) string {
		fmt.Fprintf(output, "%s", prompt)
		if defaultValue != "" {
			fmt.Fprintf(output, " [%s]", "*****") // Masked default
		}
		fmt.Fprintf(output, ": ")

		var secret string
		// Only try to read password from terminal if input is actually Stdin
		if input == os.Stdin {
			bytePassword, err := readPasswordFunc(stdinFd)
			fmt.Fprintln(output) // Newline after enter
			if err != nil {
				// Fallback or error handling
				// For now, if terminal read fails (e.g. not a TTY), try scanner
				if scanner.Scan() {
					secret = strings.TrimSpace(scanner.Text())
				}
			} else {
				secret = string(bytePassword)
			}
		} else {
			// Testing or pipe
			if scanner.Scan() {
				secret = strings.TrimSpace(scanner.Text())
			}
		}

		if secret == "" {
			return defaultValue
		}
		return secret
	}

	// 1. Provider
	currentProvider := viper.GetString("provider")
	if currentProvider == "" {
		currentProvider = "gemini"
	}
	provider := readLine("AI Provider (gemini, openai, openrouter, ollama)", currentProvider)
	viper.Set("provider", provider)

	// 2. Model
	currentModel := viper.GetString("model")
	if currentModel == "" {
		switch provider {
		case "openai":
			currentModel = "gpt-4"
		case "openrouter":
			// Default from memory or common usage
			currentModel = "google/gemini-2.5-flash-preview-09-2025"
		default:
			currentModel = "gemini-pro"
		}
	}
	model := readLine("Model", currentModel)
	viper.Set("model", model)

	// 3. API Key
	currentKey := viper.GetString("api_key")
	apiKey := readSecret("API Key", currentKey)
	viper.Set("api_key", apiKey)

	// 4. Jira Integration
	fmt.Fprintln(output, "\n--- Jira Integration (Optional) ---")
	jiraUrl := readLine("Jira URL", viper.GetString("jira.url"))
	viper.Set("jira.url", jiraUrl)

	if jiraUrl != "" {
		jiraUser := readLine("Jira Email/Username", viper.GetString("jira.username"))
		viper.Set("jira.username", jiraUser)

		jiraToken := readSecret("Jira API Token", viper.GetString("jira.api_token"))
		viper.Set("jira.api_token", jiraToken)
	}

	// Save
	fmt.Fprintln(output, "\n-----------------------")
	fmt.Fprintln(output, "Saving configuration...")

	// Force write to config.yaml in current directory if no config file used yet
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "config.yaml"
		viper.SetConfigFile(configFile)
	}

	if err := viper.WriteConfig(); err != nil {
		// If it fails, maybe it doesn't exist, try WriteConfigAs
		if err := viper.WriteConfigAs(configFile); err != nil {
			return fmt.Errorf("failed to save config to %s: %w", configFile, err)
		}
	}

	fmt.Fprintf(output, "Configuration saved to %s\n", configFile)
	fmt.Fprintln(output, "Run 'recac doctor' to verify your setup.")

	return nil
}
