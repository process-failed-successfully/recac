package manager

import (
	"context"
	"fmt"
	"log"
	"time"

	"recac/internal/jira"
	"recac/pkg/e2e/scenarios"
)

type JiraManager struct {
	Client     *jira.Client
	ProjectKey string
}

func NewJiraManager(baseURL, username, apiToken, projectKey string) *JiraManager {
	client := jira.NewClient(baseURL, username, apiToken)
	return &JiraManager{
		Client:     client,
		ProjectKey: projectKey,
	}
}

func (m *JiraManager) Authenticate(ctx context.Context) error {
	return m.Client.Authenticate(ctx)
}

func (m *JiraManager) GenerateScenario(ctx context.Context, scenarioName, repoURL string) (string, map[string]string, error) {
	scenario, ok := scenarios.Registry[scenarioName]
	if !ok {
		return "", nil, fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	uniqueID := time.Now().Format("20060102-150405")
	label := fmt.Sprintf("e2e-test-%s", uniqueID)

	// 1. Create Epic/Parent
	epicSummary := fmt.Sprintf("[E2E %s] %s", uniqueID, scenario.Name())
	epicDesc := fmt.Sprintf("%s\n\nRepo: %s", scenario.Description(), repoURL)

	epicKey, err := m.Client.CreateTicket(ctx, m.ProjectKey, epicSummary, epicDesc, "Epic", nil)
	if err != nil {
		log.Printf("Failed to create Epic, trying 'Task' as parent: %v", err)
		epicKey, err = m.Client.CreateTicket(ctx, m.ProjectKey, epicSummary, epicDesc, "Task", nil)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create Epic/Parent ticket: %w", err)
		}
	}
	fmt.Printf("Created Parent Ticket: %s\n", epicKey)
	if err := m.Client.AddLabel(ctx, epicKey, label); err != nil {
		log.Printf("Warning: Failed to add label to parent: %v", err)
	}

	// 2. Generate Tickets
	tickets := scenario.Generate(uniqueID, repoURL)
	fmt.Printf("Generating %d tickets...\n", len(tickets))

	idToKey := make(map[string]string)

	for i := range tickets {
		t := &tickets[i]
		fmt.Printf("Creating %s: %s\n", t.ID, t.Summary)
		key, err := m.Client.CreateTicket(ctx, m.ProjectKey, t.Summary, t.Desc, t.Type, nil)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create ticket %s: %w", t.ID, err)
		}
		t.JiraKey = key
		idToKey[t.ID] = key
		fmt.Printf(" -> Created %s (%s)\n", t.ID, key)

		// Add Label
		if err := m.Client.AddLabel(ctx, key, label); err != nil {
			log.Printf("Warning: Failed to add label to %s: %v", key, err)
		}

		// Link to Parent
		if err := m.Client.SetParent(ctx, key, epicKey); err != nil {
			log.Printf("Warning: Failed to set parent for %s: %v", key, err)
		}
	}

	// 3. Link Dependencies
	fmt.Println("Linking Dependencies...")
	for _, t := range tickets {
		if len(t.Blockers) == 0 {
			continue
		}
		for _, blockerID := range t.Blockers {
			blockerKey, ok := idToKey[blockerID]
			if !ok {
				log.Printf("Warning: Blocker ID %s not found for %s", blockerID, t.ID)
				continue
			}

			// BlockerKey BLOCKS t.JiraKey -> AddIssueLink(Inward=Blocked, Outward=Blocker)
			if err := m.Client.AddIssueLink(ctx, t.JiraKey, blockerKey, "Blocks"); err != nil {
				log.Printf("Failed to link %s blocks %s: %v", blockerKey, t.JiraKey, err)
			} else {
				fmt.Printf("Linked %s (%s) blocks %s (%s)\n", blockerID, blockerKey, t.ID, t.JiraKey)
			}
		}
	}

	return label, idToKey, nil
}

// Cleanup removes all tickets with the given label.
func (m *JiraManager) Cleanup(ctx context.Context, label string) error {
	issues, err := m.Client.LoadLabelIssues(ctx, label)
	if err != nil {
		return fmt.Errorf("failed to load issues for label %s: %w", label, err)
	}

	fmt.Printf("Found %d issues to delete for label %s\n", len(issues), label)
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if err := m.Client.DeleteIssue(ctx, key); err != nil {
			log.Printf("Failed to delete %s: %v", key, err)
		} else {
			fmt.Printf("Deleted %s\n", key)
		}
	}
	return nil
}
