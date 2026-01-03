package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/agent/prompts"

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
		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := getJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Test authentication
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

		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := getJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Fetch ticket
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

		// Create Jira client using factory helper
		ctx := context.Background()
		client, err := getJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		// Transition ticket
		if err := client.SmartTransition(ctx, ticketID, transition); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to transition ticket %s: %v\n", ticketID, err)
			exit(1)
		}

		fmt.Printf("Success: Ticket %s transitioned to '%s' successfully\n", ticketID, transition)
	},
}

type ticketNode struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Type        string       `json:"type"`
	Children    []ticketNode `json:"children"`
}

// jiraGenerateFromSpecCmd represents the jira generate-from-spec command
var jiraGenerateFromSpecCmd = &cobra.Command{
	Use:   "generate-from-spec",
	Short: "Generate Jira tickets from app_spec.txt",
	Long:  "Reads app_spec.txt, uses an LLM to decompose it into Epics and Stories, and creates them in Jira.",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Read app_spec.txt
		specPath, _ := cmd.Flags().GetString("spec")
		specContent, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to read spec file %s: %v\n", specPath, err)
			exit(1)
		}

		// 2. Setup Jira Client
		ctx := context.Background()
		jiraClient, err := getJiraClient(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		projectKey := os.Getenv("JIRA_PROJECT_KEY")
		if projectKey == "" {
			projectKey = viper.GetString("jira.project_key")
		}
		if projectKey == "" {
			fmt.Fprintf(os.Stderr, "Error: JIRA_PROJECT_KEY is required.\n")
			exit(1)
		}

		// 3. Setup Agent
		ag, err := getAgentClient(ctx, "", "", ".", "recac-jira-gen")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to initialize agent: %v\n", err)
			exit(1)
		}

		// 4. Generate Tickets JSON
		prompt, err := prompts.GetPrompt(prompts.TPMAgent, map[string]string{"spec": string(specContent)})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load prompt: %v\n", err)
			exit(1)
		}

		fmt.Println("Analyzing spec and generating ticket plan...")
		resp, err := ag.Send(context.Background(), prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Agent failed to generate response: %v\n", err)
			exit(1)
		}

		// Strip markdown code blocks if present
		jsonStr := resp
		if strings.Contains(jsonStr, "```json") {
			parts := strings.Split(jsonStr, "```json")
			if len(parts) > 1 {
				jsonStr = parts[1]
			}
			parts = strings.Split(jsonStr, "```")
			jsonStr = parts[0]
		} else if strings.Contains(jsonStr, "```") {
			// Generic code block
			parts := strings.Split(jsonStr, "```")
			if len(parts) > 1 {
				jsonStr = parts[1]
			}
			parts = strings.Split(jsonStr, "```")
			jsonStr = parts[0]
		}
		jsonStr = strings.TrimSpace(jsonStr)

		var tickets []ticketNode
		if err := json.Unmarshal([]byte(jsonStr), &tickets); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to parse agent response as JSON: %v\nResponse was:\n%s\n", err, resp)
			exit(1)
		}

		// 5. Create Tickets in Jira
		fmt.Printf("Found %d top-level items. Creating tickets...\n", len(tickets))

		for _, epicNode := range tickets {
			// Use the type provided by the agent (default to Epic if empty, though prompt enforces it)
			issueType := epicNode.Type
			if issueType == "" {
				issueType = "Epic"
			}

			fmt.Printf("Creating %s: %s\n", issueType, epicNode.Title)
			epicKey, err := jiraClient.CreateTicket(context.Background(), projectKey, epicNode.Title, epicNode.Description, issueType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to create %s '%s': %v\n", issueType, epicNode.Title, err)
				continue
			}
			fmt.Printf("  -> Created %s %s\n", issueType, epicKey)

			for _, storyNode := range epicNode.Children {
				childType := storyNode.Type
				if childType == "" {
					childType = "Story"
				}
				fmt.Printf("  Creating Child (%s): %s\n", childType, storyNode.Title)
				childKey, err := jiraClient.CreateChildTicket(context.Background(), projectKey, storyNode.Title, storyNode.Description, childType, epicKey)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: Failed to create child '%s': %v\n", storyNode.Title, err)
				} else {
					fmt.Printf("    -> Created %s\n", childKey)
				}
			}
		}
		fmt.Println("Done.")
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

	jiraGenerateFromSpecCmd.Flags().String("spec", "app_spec.txt", "Path to application specification file")
	jiraCmd.AddCommand(jiraGenerateFromSpecCmd)
}
