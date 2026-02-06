package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"regexp"
	"strings"
)

// generateTickets contains the core logic for ticket generation, decoupled from flags for testing.
func generateTickets(ctx context.Context, specContent, projectKey, repoURL string, allLabels []string, issueTracker IIssueTracker, ag agent.Agent) (map[string]string, error) {
	// 5. Generate Tickets JSON
	prompt, err := prompts.GetPrompt(prompts.TPMAgent, map[string]string{"spec": specContent})
	if err != nil {
		return nil, fmt.Errorf("failed to load prompt: %w", err)
	}

	fmt.Println("Analyzing spec and generating ticket plan...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate response: %w", err)
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
		return nil, fmt.Errorf("failed to parse agent response as JSON: %w\nResponse was:\n%s", err, resp)
	}

	return createTicketsFromNodes(ctx, tickets, projectKey, repoURL, allLabels, issueTracker)
}

func createTicketsFromNodes(ctx context.Context, tickets []ticketNode, projectKey, repoURL string, allLabels []string, issueTracker IIssueTracker) (map[string]string, error) {
	fmt.Printf("Found %d top-level items. Creating tickets...\n", len(tickets))

	// Validate repository in descriptions
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	// Helper for recursive validation
	var validate func([]ticketNode) error
	validate = func(nodes []ticketNode) error {
		for _, node := range nodes {
			// If repoURL is provided via flag, we don't strictly enforce it in description during validation
			// because we will inject it. But if NOT provided via flag, we enforce it.
			if repoURL == "" && !repoRegex.MatchString(node.Description) {
				return fmt.Errorf("Item '%s' description missing repository URL (Repo: https://...)", node.Title)
			}
			if err := validate(node.Children); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(tickets); err != nil {
		return nil, err
	}

	// Keep track of titles to keys for linking
	titleToKey := make(map[string]string)

	for _, node := range tickets {
		if err := createTicketRecursively(ctx, node, "", projectKey, repoURL, allLabels, issueTracker, titleToKey); err != nil {
			return nil, err
		}
	}

	// Create Links for Blockers
	fmt.Println("Creating issue links for blockers...")
	// Flatten all nodes to process links easily? Or just recurse again.
	// Let's recurse.
	var linkBlockers func([]ticketNode)
	linkBlockers = func(nodes []ticketNode) {
		for _, node := range nodes {
			nodeKey := titleToKey[node.Title]
			if nodeKey != "" {
				for _, blockerTitle := range node.BlockedBy {
					if blockerKey, ok := titleToKey[blockerTitle]; ok {
						fmt.Printf("Linking %s as blocked by %s\n", nodeKey, blockerKey)
						if err := issueTracker.AddIssueLink(ctx, blockerKey, nodeKey, "Blocks"); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: Failed to link %s as blocked by %s: %v\n", nodeKey, blockerKey, err)
						}
					}
				}
			}
			linkBlockers(node.Children)
		}
	}
	linkBlockers(tickets)

	// 4. Map logical IDs back from titles
	idToKey := make(map[string]string)
	idRegex := regexp.MustCompile(`(?i)ID:\[?([\w-]+)\]?`) // Match ID:[SQL] or ID:SQL-1

	for title, key := range titleToKey {
		matches := idRegex.FindStringSubmatch(title)
		if len(matches) > 1 {
			idToKey[matches[1]] = key
			fmt.Printf("Mapped ID %s -> %s\n", matches[1], key)
		}
	}

	fmt.Println("Done.")
	return idToKey, nil
}

func createTicketRecursively(ctx context.Context, node ticketNode, parentKey, projectKey, repoURL string, allLabels []string, issueTracker IIssueTracker, titleToKey map[string]string) error {
	issueType := node.Type
	if issueType == "" {
		// Inference fallback
		if parentKey == "" {
			issueType = "Epic"
		} else {
			issueType = "Story" // Default child
		}
	}

	indent := ""
	if parentKey != "" {
		indent = "  "
	}

	fmt.Printf("%sCreating %s: %s\n", indent, issueType, node.Title)

	// Combine Description and Acceptance Criteria
	fullDescription := node.Description
	if len(node.AcceptanceCriteria) > 0 {
		fullDescription += "\n\nAcceptance Criteria:\n"
		for _, ac := range node.AcceptanceCriteria {
			fullDescription += fmt.Sprintf("- %s\n", ac)
		}
	}

	// Inject Repo URL if provided and missing
	if repoURL != "" && !strings.Contains(strings.ToLower(fullDescription), "repo: http") {
		fullDescription += fmt.Sprintf("\n\nRepo: %s", repoURL)
	}

	var key string
	var err error

	if parentKey == "" {
		// Top level
		key, err = issueTracker.CreateTicket(ctx, projectKey, node.Title, fullDescription, issueType, allLabels)
	} else {
		// Child
		key, err = issueTracker.CreateChildTicket(ctx, projectKey, node.Title, fullDescription, issueType, parentKey, allLabels)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create %s '%s': %v. Trying 'Task'...\n", issueType, node.Title, err)
		fallbackType := "Task"
		// If it's a subtask level, maybe we need "Subtask"? But CreateChildTicket might handle that mapping if the provider needs it.
		// For now assuming "Task" is a safe fallback for Stories, but Sub-tasks are special in Jira.
		// If explicit "Subtask" failed, falling back to "Task" might fail if parent is an issue that can't have "Task" as subtask?
		// Actually typical Jira hierarchy: Epic -> Story/Task/Bug -> Subtask.
		// If we are at level 3 (Subtask), "Task" might not be valid sub-issue type.
		// But let's assume the user config/Jira setup handles standard types.

		if parentKey == "" {
			key, err = issueTracker.CreateTicket(ctx, projectKey, node.Title, fullDescription, fallbackType, allLabels)
		} else {
			key, err = issueTracker.CreateChildTicket(ctx, projectKey, node.Title, fullDescription, fallbackType, parentKey, allLabels)
		}

		if err != nil {
			return fmt.Errorf("failed to create ticket '%s': %w", node.Title, err)
		}
		issueType = fallbackType // update for log
	}

	fmt.Printf("%s-> Created %s %s\n", indent, issueType, key)
	titleToKey[node.Title] = key

	for _, child := range node.Children {
		if err := createTicketRecursively(ctx, child, key, projectKey, repoURL, allLabels, issueTracker, titleToKey); err != nil {
			return err
		}
	}
	return nil
}
