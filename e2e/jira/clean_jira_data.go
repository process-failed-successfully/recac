//go:build ignore

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"recac/internal/jira"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	var label string
	var force bool
	var all bool
	flag.StringVar(&label, "label", "", "Jira label to clean up (e.g. e2e-test-20240101)")
	flag.BoolVar(&force, "force", false, "Force delete without confirmation")
	flag.BoolVar(&all, "all", false, "Delete ALL tickets in ALL projects (requires --force)")
	flag.Parse()

	if label == "" && !all {
		log.Fatal("Error: --label or --all is required")
	}

	if all && !force {
		log.Fatal("Error: --all requires --force to be set")
	}

	baseURL := os.Getenv("JIRA_URL")
	username := os.Getenv("JIRA_USERNAME")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_API_KEY")
	}

	if baseURL == "" || username == "" || apiToken == "" {
		log.Fatal("Missing required environment variables: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN")
	}

	client := jira.NewClient(baseURL, username, apiToken)
	ctx := context.Background()

	// 1. Find Issues
	var issues []map[string]interface{}
	var err error

	if all {
		fmt.Println("WARNING: Fetching ALL issues in the instance...")
		// Fetch all issues. Using "order by created DESC" to just get a list.
		// We might need to handle pagination if there are many, but SearchIssues usually handles some default limit (often 50 or 100).
		// If the user wants to clean *everything*, we should probably loop until empty, or increase limit.
		// For now, let's assume the client's SearchIssues does a reasonable fetch or the user will run it multiple times.
		// Ideally we'd use "ORDER BY created DESC" to get them.
		jql := "created is not EMPTY ORDER BY created DESC"
		issues, err = client.SearchIssues(ctx, jql)
		if err != nil {
			log.Fatalf("Failed to search all issues: %v", err)
		}
	} else if strings.Contains(label, "*") {
		fmt.Printf("Searching for issues with label: %s...\n", label)
		// Wildcard search
		prefix := strings.TrimSuffix(label, "*")
		fmt.Printf("Wildcard detected. Searching for issues with labels starting with: %s\n", prefix)

		// Construct JQL to find candidates.
		// We look for any issue with labels (labels is not EMPTY) to iterate and filter.
		// Optimization: If we have a project key, limit to it.
		projectKey := os.Getenv("JIRA_PROJECT_KEY")
		jql := "labels is not EMPTY"
		if projectKey != "" {
			jql += fmt.Sprintf(" AND project = \"%s\"", projectKey)
		}

		fmt.Printf("JQL: %s\n", jql)
		candidates, err := client.SearchIssues(ctx, jql)
		if err != nil {
			log.Fatalf("Failed to search issues: %v", err)
		}

		// Filter locally
		for _, issue := range candidates {
			fields, ok := issue["fields"].(map[string]interface{})
			if !ok {
				continue
			}
			labels, ok := fields["labels"].([]interface{})
			if !ok {
				continue
			}

			matched := false
			for _, l := range labels {
				if lStr, ok := l.(string); ok {
					if strings.HasPrefix(lStr, prefix) {
						matched = true
						break
					}
				}
			}
			if matched {
				issues = append(issues, issue)
			}
		}

	} else {
		fmt.Printf("Searching for issues with label: %s...\n", label)
		// Exact match
		issues, err = client.LoadLabelIssues(ctx, label)
		if err != nil {
			log.Fatalf("Failed to search issues: %v", err)
		}
	}

	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	fmt.Printf("Found %d issues to delete:\n", len(issues))
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		fields, _ := issue["fields"].(map[string]interface{})
		summary, _ := fields["summary"].(string)
		fmt.Printf("- %s: %s\n", key, summary)
	}

	// 2. Confirm
	if !force {
		fmt.Print("\nAre you sure you want to delete these issues? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	// 3. Delete
	fmt.Println("\nDeleting issues...")
	count := 0
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if err := client.DeleteIssue(ctx, key); err != nil {
			log.Printf("Failed to delete %s: %v", key, err)
		} else {
			fmt.Printf("Deleted %s\n", key)
			count++
		}
	}

	fmt.Printf("\nCleanup complete. Deleted %d/%d issues.\n", count, len(issues))
}
