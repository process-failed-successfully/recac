package main

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/jira"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a series of checks to diagnose issues.",
	Long:  `Run a series of checks to diagnose issues with your recac setup, including configuration, Docker, and API connectivity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🩺 Running recac doctor...")

		allChecksPassed := true

		if !checkConfiguration() {
			allChecksPassed = false
		}
		if !checkDocker() {
			allChecksPassed = false
		}
		if !checkJira() {
			allChecksPassed = false
		}
		if !checkAIProvider() {
			allChecksPassed = false
		}

		fmt.Println()
		if allChecksPassed {
			color.Green("✅ All checks passed! Your recac setup is ready.")
		} else {
			color.Red("❌ Some checks failed. Please review the output above for details.")
		}

		return nil
	},
}

func checkConfiguration() bool {
	fmt.Print("  Checking configuration... ")
	if viper.ConfigFileUsed() == "" {
		printCheckResult(false, "No configuration file found.")
		return false
	}

	printCheckResult(true, fmt.Sprintf("Using config file: %s", viper.ConfigFileUsed()))
	return true
}

func checkDocker() bool {
	fmt.Print("  Checking Docker daemon... ")
	client, err := docker.NewClient("doctor-check")
	if err != nil {
		printCheckResult(false, fmt.Sprintf("Failed to create docker client: %v", err))
		return false
	}
	defer client.Close()

	if err := client.CheckDaemon(context.Background()); err != nil {
		printCheckResult(false, fmt.Sprintf("Docker daemon is not reachable: %v", err))
		return false
	}

	printCheckResult(true, "Docker daemon is running and accessible.")
	return true
}

func checkJira() bool {
	fmt.Print("  Checking Jira connectivity... ")
	if viper.GetString("jira_url") == "" {
		printCheckResult(true, "Jira not configured, skipping.")
		return true
	}

	jiraClient := jira.NewClient(viper.GetString("jira_url"), viper.GetString("jira_email"), viper.GetString("jira_token"))
	if err := jiraClient.Authenticate(context.Background()); err != nil {
		printCheckResult(false, fmt.Sprintf("Jira authentication failed: %v", err))
		return false
	}

	printCheckResult(true, "Jira connection successful.")
	return true
}

func checkAIProvider() bool {
	fmt.Print("  Checking AI provider connectivity... ")
	provider := viper.GetString("agent_provider")
	apiKey := viper.GetString("api_key")
	model := viper.GetString("agent_model")

	if provider == "" {
		printCheckResult(false, "agent_provider is not set in configuration.")
		return false
	}

	agent, err := agent.NewAgent(provider, apiKey, model, "", "doctor-check")
	if err != nil {
		printCheckResult(false, fmt.Sprintf("Failed to create agent: %v", err))
		return false
	}

	_, err = agent.Send(context.Background(), "hello")
	if err != nil {
		printCheckResult(false, fmt.Sprintf("AI provider connection failed: %v", err))
		return false
	}

	printCheckResult(true, "AI provider connection successful.")
	return true
}

func printCheckResult(success bool, message string) {
	if success {
		color.Green("✔")
		fmt.Printf("  %s\n", message)
	} else {
		color.Red("✖")
		fmt.Printf("\n      Error: %s\n", message)
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
