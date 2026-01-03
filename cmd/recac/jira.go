package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/jira"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// jiraCmd represents the jira command
var jiraCmd = &cobra.Command{
	Use:   "jira",
	Short: "Jira integration commands",
	Long:  "Commands for interacting with Jira API",
}

// jiraTestAuthCmd represents the jira test-auth command
var jiraTestAuthCmd = &cobra.Command{
	Use:   "test-auth",
	Short: "Test Jira authentication",
	Long: `Test Jira authentication using credentials from environment variables or config.
	
Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get credentials from environment variables (preferred) or config
		baseURL := os.Getenv("JIRA_URL")
		if baseURL == "" {
			baseURL = viper.GetString("jira.url")
		}

		username := os.Getenv("JIRA_USERNAME")
		if username == "" {
			username = viper.GetString("jira.username")
		}

		apiToken := os.Getenv("JIRA_API_TOKEN")
		if apiToken == "" {
			apiToken = viper.GetString("jira.api_token")
		}

		// Validate required fields
		if baseURL == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_URL environment variable or jira.url config is required\n")
			exit(1)
		}
		if username == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_USERNAME environment variable or jira.username config is required\n")
			exit(1)
		}
		if apiToken == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_API_TOKEN environment variable or jira.api_token config is required\n")
			exit(1)
		}

		// Create Jira client
		client := jira.NewClient(baseURL, username, apiToken)

		// Test authentication
		ctx := context.Background()
		if err := client.Authenticate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Authentication failed: %v\n", err)
			exit(1)
		}

		fmt.Println("Success: Jira authentication successful!")
	},
}

// jiraGetCmd represents the jira get command
var jiraGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a Jira ticket by ID",
	Long: `Fetch and display a Jira ticket by its ID (e.g., PROJ-123).

Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get ticket ID from flag
		ticketID, _ := cmd.Flags().GetString("id")
		if ticketID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id flag is required\n")
			fmt.Fprintf(os.Stderr, "Usage: %s jira get --id PROJ-123\n", os.Args[0])
			exit(1)
		}

		// Get credentials from environment variables (preferred) or config
		baseURL := os.Getenv("JIRA_URL")
		if baseURL == "" {
			baseURL = viper.GetString("jira.url")
		}

		username := os.Getenv("JIRA_USERNAME")
		if username == "" {
			username = viper.GetString("jira.username")
		}

		apiToken := os.Getenv("JIRA_API_TOKEN")
		if apiToken == "" {
			apiToken = viper.GetString("jira.api_token")
		}

		// Validate required fields
		if baseURL == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_URL environment variable or jira.url config is required\n")
			exit(1)
		}
		if username == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_USERNAME environment variable or jira.username config is required\n")
			exit(1)
		}
		if apiToken == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_API_TOKEN environment variable or jira.api_token config is required\n")
			exit(1)
		}

		// Create Jira client
		client := jira.NewClient(baseURL, username, apiToken)

		// Fetch ticket
		ctx := context.Background()
		ticket, err := client.GetTicket(ctx, ticketID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to fetch ticket %s: %v\n", ticketID, err)
			exit(1)
		}

		// Extract and display ticket details
		key, _ := ticket["key"].(string)
		fields, ok := ticket["fields"].(map[string]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: Invalid ticket response format\n")
			exit(1)
		}

		summary, _ := fields["summary"].(string)
		description := client.ParseDescription(ticket)

		// Display ticket information
		fmt.Printf("Ticket: %s\n", key)
		fmt.Printf("Title: %s\n", summary)
		if description != "" {
			fmt.Printf("Description:\n%s\n", description)
		} else {
			fmt.Println("Description: (empty)")
		}
	},
}

// jiraTransitionCmd represents the jira transition command
var jiraTransitionCmd = &cobra.Command{
	Use:   "transition",
	Short: "Transition a Jira ticket to a new status",
	Long: `Transition a Jira ticket to a new status (e.g., "In Progress").

Environment variables:
  JIRA_URL       - Jira instance URL (e.g., https://yourcompany.atlassian.net)
  JIRA_USERNAME  - Jira username or email
  JIRA_API_TOKEN - Jira API token
  
Or configure in config.yaml:
  jira:
    url: https://yourcompany.atlassian.net
    username: user@example.com
    api_token: your-api-token`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get ticket ID from flag
		ticketID, _ := cmd.Flags().GetString("id")
		if ticketID == "" {
			fmt.Fprintf(os.Stderr, "Error: --id flag is required\n")
			fmt.Fprintf(os.Stderr, "Usage: %s jira transition --id PROJ-123 --transition-id 31\n", os.Args[0])
			exit(1)
		}

		// Get transition Name or ID from flag (defaults to "In Progress")
		transition, _ := cmd.Flags().GetString("transition")
		if transition == "" {
			transition = "In Progress"
		}

		// Get credentials from environment variables (preferred) or config
		baseURL := os.Getenv("JIRA_URL")
		if baseURL == "" {
			baseURL = viper.GetString("jira.url")
		}

		username := os.Getenv("JIRA_USERNAME")
		if username == "" {
			username = viper.GetString("jira.username")
		}

		apiToken := os.Getenv("JIRA_API_TOKEN")
		if apiToken == "" {
			apiToken = viper.GetString("jira.api_token")
		}

		// Validate required fields
		if baseURL == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_URL environment variable or jira.url config is required\n")
			exit(1)
		}
		if username == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_USERNAME environment variable or jira.username config is required\n")
			exit(1)
		}
		if apiToken == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_API_TOKEN environment variable or jira.api_token config is required\n")
			exit(1)
		}

		// Create Jira client
		client := jira.NewClient(baseURL, username, apiToken)

		// Transition ticket
		ctx := context.Background()
		if err := client.SmartTransition(ctx, ticketID, transition); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to transition ticket %s: %v\n", ticketID, err)
			exit(1)
		}

		fmt.Printf("Success: Ticket %s transitioned to '%s' successfully\n", ticketID, transition)
	},
}

func init() {
	rootCmd.AddCommand(jiraCmd)
	jiraCmd.AddCommand(jiraTestAuthCmd)
	jiraGetCmd.Flags().String("id", "", "Jira ticket ID (e.g., PROJ-123)")
	jiraGetCmd.MarkFlagRequired("id")
	jiraCmd.AddCommand(jiraGetCmd)
	jiraTransitionCmd.Flags().String("id", "", "Jira ticket ID (e.g., PROJ-123)")
	jiraTransitionCmd.Flags().String("transition", "", "Transition Name or ID (defaults to 'In Progress')")
	jiraTransitionCmd.MarkFlagRequired("id")
	jiraCmd.AddCommand(jiraTransitionCmd)
}
