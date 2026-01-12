package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/docker"
	"recac/internal/jira"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// check represents a single diagnostic check
type check struct {
	Name        string
	Description string
	Run         func() (bool, error)
	Remedy      string
}

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a series of checks to diagnose issues with your environment",
	Long: `Doctor checks for common problems with your configuration,
dependencies like Docker, and connectivity to external services.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChecks()
	},
}

// runChecks executes all diagnostic checks and prints a summary report.
func runChecks() error {
	checks := []check{
		{
			Name:        "Docker Connectivity",
			Description: "Checking if the Docker daemon is running and accessible.",
			Run:         checkDocker,
			Remedy:      "Ensure the Docker daemon is running.",
		},
		{
			Name:        "Config File",
			Description: "Checking for a valid configuration file.",
			Run:         checkConfig,
			Remedy:      "Make sure you have a .recac.yaml file in your home directory or provide a config file using --config.",
		},
		{
			Name:        "API Key",
			Description: "Checking for a configured agent provider and API key.",
			Run:         checkAPIKey,
			Remedy:      "Set 'agent_provider' and 'api_key' in your config file.",
		},
		{
			Name:        "Jira Connectivity",
			Description: "Checking the connection to Jira.",
			Run:         checkJira,
			Remedy:      "Verify your Jira URL, username, and API token in the config file.",
		},
	}

	fmt.Println("🩺 Running recac doctor...")
	allPassed := true

	for _, c := range checks {
		fmt.Printf("\n🔎 %s\n", c.Name)
		fmt.Printf("   %s\n", c.Description)
		passed, err := c.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ❌ Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "   💡 Remedy: %s\n", c.Remedy)
			allPassed = false
		} else if !passed {
			fmt.Fprintln(os.Stderr, "   ❌ Check failed.")
			allPassed = false
		} else {
			fmt.Println("   ✅ Check passed.")
		}
	}

	fmt.Println("\n---")
	if allPassed {
		fmt.Println("✅ All checks passed! Your environment is ready.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "❌ Some checks failed. Please review the output above and address the issues.")
	return fmt.Errorf("doctor checks failed")
}

// checkDocker verifies that the Docker daemon is running and responsive.
func checkDocker() (bool, error) {
	client, err := docker.NewClient("doctor")
	if err != nil {
		return false, fmt.Errorf("could not create docker client: %w", err)
	}
	defer client.Close()

	if err := client.CheckDaemon(context.Background()); err != nil {
		return false, err
	}
	return true, nil
}

// checkConfig checks if a configuration file is loaded and has essential keys.
func checkConfig() (bool, error) {
	if viper.ConfigFileUsed() == "" {
		return false, fmt.Errorf("no config file found")
	}
	fmt.Printf("   ℹ️  Using config file: %s\n", viper.ConfigFileUsed())
	return true, nil
}

// checkAPIKey checks if the agent provider and API key are configured.
func checkAPIKey() (bool, error) {
	if viper.GetString("agent_provider") == "" {
		return false, fmt.Errorf("'agent_provider' is not set")
	}
	if viper.GetString("api_key") == "" {
		return false, fmt.Errorf("'api_key' is not set")
	}
	return true, nil
}

// checkJira tests the connection to the configured Jira instance.
func checkJira() (bool, error) {
	if viper.GetString("jira.url") == "" {
		// Jira is optional, so we skip if not configured.
		fmt.Println("   ⚪ Jira is not configured, skipping check.")
		return true, nil
	}

	client, err := jira.NewClient(
		viper.GetString("jira.url"),
		viper.GetString("jira.email"),
		viper.GetString("jira.api_token"),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create Jira client: %w", err)
	}

	if err := client.Authenticate(context.Background()); err != nil {
		return false, fmt.Errorf("Jira authentication failed: %w", err)
	}
	return true, nil
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
