//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"recac/internal/jira"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	var issueKey string
	flag.StringVar(&issueKey, "issue", "", "Jira Issue Key to inspect (e.g. PROJ-123)")
	flag.Parse()

	if issueKey == "" {
		log.Fatal("Error: --issue is required")
	}

	baseURL := os.Getenv("JIRA_URL")
	username := os.Getenv("JIRA_USERNAME")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		fallback := os.Getenv("JIRA_API_KEY")
		if fallback != "" {
			apiToken = fallback
		}
	}

	if baseURL == "" || username == "" || apiToken == "" {
		log.Fatal("Missing required environment variables: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN")
	}

	client := jira.NewClient(baseURL, username, apiToken)
	ctx := context.Background()

	fmt.Printf("Fetching transitions for %s...\n", issueKey)
	transitions, err := client.GetTransitions(ctx, issueKey)
	if err != nil {
		log.Fatalf("Failed to get transitions: %v", err)
	}

	if len(transitions) == 0 {
		fmt.Println("No transitions available for this issue.")
		return
	}

	fmt.Println("Available Transitions:")
	for _, t := range transitions {
		name, _ := t["name"].(string)
		id, _ := t["id"].(string)
		to, _ := t["to"].(map[string]interface{})
		toStatus, _ := to["name"].(string)

		fmt.Printf("- %s (ID: %s) -> To Status: %s\n", name, id, toStatus)
	}
}
