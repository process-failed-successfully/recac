//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"recac/pkg/e2e/manager"
	"recac/pkg/e2e/scenarios"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Flags
	var scenarioName string
	flag.StringVar(&scenarioName, "scenario", "http-proxy", "Name of the scenario to generate")
	flag.Parse()

	// Configuration
	baseURL := os.Getenv("JIRA_URL")
	username := os.Getenv("JIRA_USERNAME")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_API_KEY")
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")

	if baseURL == "" || username == "" || apiToken == "" {
		log.Fatal("Missing required environment variables: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN")
	}

	mgr := manager.NewJiraManager(baseURL, username, apiToken, projectKey)
	ctx := context.Background()

	if err := mgr.Authenticate(ctx); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e"
	label, err := mgr.GenerateScenario(ctx, scenarioName, repoURL)
	if err != nil {
		log.Fatalf("Failed to generate scenario: %v", err)
	}

	fmt.Printf("\nDone! Use label: %s\n", label)

	// Output for GitHub Actions / Scripts
	if githubOutput := os.Getenv("GITHUB_OUTPUT"); githubOutput != "" {
		f, err := os.OpenFile(githubOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			fmt.Fprintf(f, "jira_label=%s\n", label)
			f.Close()
		}
	}
}
