package main

import (
	"github.com/spf13/cobra"
)

// ticketsCmd represents the tickets command
var ticketsCmd = &cobra.Command{
	Use:   "tickets",
	Short: "Generate Jira tickets from app_spec.txt (alias for 'jira generate-from-spec')",
	Long:  "Reads app_spec.txt, uses an LLM to decompose it into Epics and Stories, and creates them in Jira.",
	Run:   runGenerateTicketsCmd,
}

func init() {
	rootCmd.AddCommand(ticketsCmd)

	ticketsCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	ticketsCmd.Flags().String("project", "", "Jira project key (overrides JIRA_PROJECT_KEY env var and config)")
	ticketsCmd.Flags().StringSliceP("label", "l", []string{}, "Custom labels to add to generated tickets")
}
