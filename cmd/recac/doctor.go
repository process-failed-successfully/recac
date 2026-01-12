package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"recac/internal/docker"
	"recac/internal/jira"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a series of checks to diagnose issues.",
	Long:  `Run a series of checks to diagnose issues with your recac setup, including configuration, Docker, and API connectivity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running recac doctor...")

		// Check 1: Configuration
		checkConfiguration()

		// Check 2: Docker
		checkDocker()

		// Check 3: Jira
		checkJira()

		// Check 4: AI Provider
		checkAIProvider()

		return nil
	},
}

func checkConfiguration() {
	fmt.Print("  [ ] Checking configuration... ")
	// TODO: Implement configuration check
	printCheckResult(true, "Configuration check not yet implemented.")
}

func checkDocker() {
	fmt.Print("  [ ] Checking Docker daemon... ")
	// TODO: Implement Docker check
	printCheckResult(true, "Docker check not yet implemented.")
}

func checkJira() {
	fmt.Print("  [ ] Checking Jira connectivity... ")
	// TODO: Implement Jira check
	printCheckResult(true, "Jira check not yet implemented.")
}

func checkAIProvider() {
	fmt.Print("  [ ] Checking AI provider connectivity... ")
	// TODO: Implement AI provider check
	printCheckResult(true, "AI provider check not yet implemented.")
}


func printCheckResult(success bool, message string) {
	if success {
		color.Green("✔")
		if message != "" {
			fmt.Printf("      %s\n", message)
		}
	} else {
		color.Red("✖")
		if message != "" {
			fmt.Printf("      Error: %s\n", message)
		}
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
