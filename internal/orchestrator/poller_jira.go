package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"recac/internal/jira"
)

// JiraClient defines the interface for the Jira client used by the poller.
// This allows for mocking in tests.
type JiraClient interface {
	SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error)
	GetBlockers(issue map[string]interface{}) []string
	ParseDescription(issue map[string]interface{}) string
	SmartTransition(ctx context.Context, issueID string, status string) error
	AddComment(ctx context.Context, issueID, comment string) error
}

type JiraPoller struct {
	Client  JiraClient
	JQL     string
	Label   string // Helper to construct JQL if JQL not provided
	Project string // Helper to construct JQL
}

func NewJiraPoller(client JiraClient, jql string) *JiraPoller {
	return &JiraPoller{
		Client: client,
		JQL:    jql,
	}
}

func (p *JiraPoller) Poll(ctx context.Context) ([]WorkItem, error) {
	// Default JQL if empty
	if p.JQL == "" {
		p.JQL = "statusCategory != Done ORDER BY created ASC"
	}

	fmt.Println("DEBUG: SearchIssues start")
	issues, err := p.Client.SearchIssues(ctx, p.JQL)
	fmt.Println("DEBUG: SearchIssues done", len(issues), err)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	if len(issues) == 0 {
		return nil, nil // No work
	}

	// Build Dependency Graph to find actionable items
	fmt.Println("DEBUG: BuildGraph start")
	graph := jira.BuildGraphFromIssues(issues, func(issue map[string]interface{}) []string {
		raw := p.Client.GetBlockers(issue)
		keys := make([]string, 0, len(raw))
		for _, r := range raw {
			// Format "KEY (Status)"
			parts := strings.Split(r, " (")
			if len(parts) > 0 {
				keys = append(keys, parts[0])
			}
		}
		return keys
	})

	// We only want items that are READY (no local blockers).
	// We assume external blockers (not in 'issues') might also exist?
	// jira.BuildGraphFromIssues filters dependencies to only those in the set.
	// This means if A depends on Z, and Z is NOT in 'issues', A is considered READY by the graph.
	// However, GetBlockers returns "KEY (Status)" for NON-DONE blockers.
	// So if Z is returned by GetBlockers, it is NOT Done.
	// If Z is not in our JQL result, it means we didn't fetch it, but it blocks A.
	// So A should NOT run.
	// We need to handle "External Blockers".
	// The current `jira.BuildGraphFromIssues` ignores external blockers.
	// We should probably check if `GetBlockers` returned ANY blockers, and if so, don't run.
	// Let's refine the ready check.

	// Actually, `GetBlockers` returns "tickets that block ... and are not Done".
	// So if `GetBlockers` returns non-empty, the ticket is blocked by SOMETHING that is not done.
	// So it is not ready.

	var curatedItems []WorkItem
	// We use the graph for ordering/sorting, but simple "IsBlocked" check is safer?
	// The graph helps if we have A -> B in the SAME batch, so we don't start B until A is done.
	// But `Poll` returns a list of items to process NOW.
	// If we return [A, B] and A blocks B, the Orchestrator spawns them in parallel.
	// The Orchestrator does NOT wait for A before spawning B in the current implementation.
	// The Orchestrator spawns EVERYTHING returned by Poll.
	// So Poll MUST ONLY return items that are truly ready to run immediately.
	// So if A blocks B, we should return A. Next Poll, if A is done, we return B.

	// So we need `GetReadyTickets` but we also need to respect that we can't run B yet.
	// `graph.GetReadyTickets` handles internal dependencies.
	// But what about A? It will be in `ready`.
	// If A requires time to run, B shouldn't start.
	// So returning both A and B (if B didn't depend on A) is fine.
	// But if B depends on A, `GetReadyTickets` will NOT return B because A is not in `completed`.
	// So `GetReadyTickets` with empty `completed` map returns only roots.
	// This ensures we don't start B.
	// AND we also need to check for External Blockers.

	readyKeys := graph.GetReadyTickets(nil) // Empty completed set

	// Filter readyKeys for external blockers too (safe guard)
	finalKeys := make([]string, 0, len(readyKeys))
	issueMap := make(map[string]map[string]interface{})
	for _, i := range issues {
		k, _ := i["key"].(string)
		issueMap[k] = i
	}

	for _, key := range readyKeys {
		issue := issueMap[key]
		blockers := p.Client.GetBlockers(issue)
		if len(blockers) > 0 {
			// This ticket has blockers (internal or external) that are not Done.
			// Since GetReadyTickets(nil) says it's ready internally, it means:
			// no internal dependency blocks it.
			// But if len(blockers) > 0, it must be an EXTERNAL blocker (or a cycle? or self?).
			// Wait, if it has internal blockers, GetReadyTickets WOULD filter it out.
			// So if it's here, blockers must be external.
			// If external blockers exist and are not Done, we shouldn't run.
			continue
		}
		finalKeys = append(finalKeys, key)
	}

	// Construct WorkItems
	for _, key := range finalKeys {
		issue := issueMap[key]
		fields, _ := issue["fields"].(map[string]interface{})
		summary, _ := fields["summary"].(string)
		description := p.Client.ParseDescription(issue)

		// Extract Repo
		repoURL := extractRepoURL(description)

		// If no Repo found, we can't run agent really.
		// Unless we allow no-repo agents?
		// For now, require repo or skip/log.
		if repoURL == "" {
			// Maybe log warning?
			continue
		}

		item := WorkItem{
			ID:          key,
			Summary:     summary,
			Description: description,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"JIRA_TICKET": key,
			},
		}
		curatedItems = append(curatedItems, item)
	}

	return curatedItems, nil
}

func (p *JiraPoller) Claim(ctx context.Context, item WorkItem) error {
	// Transition to "In Progress"
	return p.Client.SmartTransition(ctx, item.ID, "In Progress")
}

func (p *JiraPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	if comment != "" {
		_ = p.Client.AddComment(ctx, item.ID, comment)
	}
	// Map status to transition?
	// This might be fuzzy. "Failed", "Done", etc.
	if status != "" {
		// Only attempt transition if mapped clearly.
		// TODO: Configurable mapping
		return p.Client.SmartTransition(ctx, item.ID, status)
	}
	return nil
}

func extractRepoURL(text string) string {
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	matches := repoRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSuffix(matches[1], ".git")
	}
	return ""
}
