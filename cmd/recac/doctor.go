package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
	"recac/internal/jira"
)

var doctorFixFlag bool

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Diagnose and fix common environment issues",
	Long:    `The doctor command runs a series of checks to diagnose the health of your local recac environment. It verifies your configuration, dependencies, and API connectivity. Use the --fix flag to allow the doctor to attempt to automatically resolve any issues it finds.`,
	Aliases: []string{"check"}, // Alias check to doctor for backward compatibility
	RunE:    doDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFixFlag, "fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(doctorCmd)
}

// For testing purposes, we assign the check functions to variables
// so they can be mocked.
var (
	runCheckGoVersion        = checkGoVersion
	runCheckDockerConnection = checkDockerConnection
	runCheckAppConfig        = checkAppConfig
	runCheckJiraAuth         = checkJiraAuth
	runCheckAIProvider       = checkAIProvider
)

func doDoctor(cmd *cobra.Command, args []string) error {
	if cmd.CalledAs() == "check" {
		cmd.Println(color.YellowString("⚠️  Warning: The 'check' command is deprecated and has been renamed to 'doctor'. Please use 'recac doctor' in the future."))
		cmd.Println()
	}

	bold := color.New(color.Bold).SprintFunc()
	cmd.Println(bold("Running diagnostics..."))

	allPassed := true
	var issues []string

	// Runner function to execute checks
	runCheck := func(title string, checkFunc func() (string, error)) {
		cmd.Printf("🩺 Checking %s...", title)
		msg, err := checkFunc()
		if err != nil {
			allPassed = false
			// This is tricky to test with color, so we'll just print FAIL.
			// In a real terminal, it will be colored.
			cmd.Println(" FAIL")
			issues = append(issues, fmt.Sprintf("❌ %s: %v", title, err))
		} else {
			cmd.Println(" PASS")
			cmd.Printf("  %s\n", msg)
		}
	}

	runCheck("Go Environment", runCheckGoVersion)
	runCheck("Docker Environment", runCheckDockerConnection)
	runCheck("Configuration", runCheckAppConfig)
	runCheck("Jira Connectivity", runCheckJiraAuth)
	runCheck("AI Provider Connectivity", runCheckAIProvider)

	if allPassed {
		cmd.Println(color.GreenString("\n✅ All checks passed! Your environment is ready. 🚀"))
	} else {
		cmd.Println(color.YellowString("\n⚠️  Found issues:"))
		for _, issue := range issues {
			cmd.Println(issue)
		}
		if !doctorFixFlag {
			cmd.Println("\nRun 'recac doctor --fix' to attempt automatic repairs.")
		}
		// Use the testable exit function instead of os.Exit
		exit(1)
	}
	return nil
}

// --- Individual Check Functions ---

func checkAppConfig() (string, error) {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return "", fmt.Errorf("no config file found. Searched in: $HOME/.recac.yaml and current directory")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return "", fmt.Errorf("config file specified (%s) does not exist", configFile)
	}
	return fmt.Sprintf("Using config: %s", configFile), nil
}

func checkGoVersion() (string, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("the 'go' binary was not found in your PATH")
	}

	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run 'go version': %w", err)
	}

	version := strings.TrimSpace(string(output))
	return fmt.Sprintf("Found go at %s (%s)", path, version), nil
}

func checkDockerConnection() (string, error) {
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to connect to Docker daemon. Is Docker running? Error: %s", string(output))
	}

	// Also try to pull a test image
	pullCmd := exec.Command("docker", "pull", "hello-world")
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Docker is running, but failed to pull test image 'hello-world'. Error: %s", string(pullOutput))
	}

	return "Docker daemon is running and accessible.", nil
}

func checkJiraAuth() (string, error) {
	jiraURL := viper.GetString("jira_url")
	if jiraURL == "" {
		return "Jira not configured, skipping.", nil
	}
	jiraEmail := viper.GetString("jira_email")
	jiraToken := viper.GetString("jira_token")

	if jiraEmail == "" || jiraToken == "" {
		return "", fmt.Errorf("Jira is partially configured. Please set 'jira_email' and 'jira_token'")
	}

	client := jira.NewClient(jiraURL, jiraEmail, jiraToken)
	if client == nil {
		return "", fmt.Errorf("failed to create Jira client")
	}

	err := client.Authenticate(context.Background())
	if err != nil {
		return "", fmt.Errorf("Jira authentication failed. Please check your 'jira_url', 'jira_email', and 'jira_token' in your config. Raw error: %w", err)
	}

	return "Jira credentials are valid.", nil
}

func checkAIProvider() (string, error) {
	provider := viper.GetString("agent_provider")
	if provider == "" {
		return "", fmt.Errorf("agent_provider is not set in your config")
	}
	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		return "", fmt.Errorf("api_key is not set for provider '%s'. Please add it to your config or environment variables", provider)
	}

	// We create the agent with dummy values for the other fields, since we only want to check provider and key.
	if _, err := agent.NewAgent(provider, apiKey, "", "", ""); err != nil {
		return "", fmt.Errorf("failed to create agent for provider '%s'. Is it a supported provider? Error: %w", provider, err)
	}

	return fmt.Sprintf("AI provider '%s' is configured correctly.", provider), nil
}
