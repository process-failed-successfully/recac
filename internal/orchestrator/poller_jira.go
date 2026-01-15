package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"recac/internal/jira"
	"regexp"
	"strings"
)

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

func (p *JiraPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	// Default JQL if empty
	if p.JQL == "" {
		p.JQL = "statusCategory != Done ORDER BY created ASC"
	}

	issues, err := p.Client.SearchIssues(ctx, p.JQL)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	if len(issues) == 0 {
		return nil, nil // No work
	}

	// Build Dependency Graph to find actionable items
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

	// We only want items that are READY (no local blockers and no external blockers).
	// GetReadyTickets(nil) returns items with no internal dependencies in the current set.
	var curatedItems []WorkItem

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
		// If blockers exist (internal or external), skip.
		// GetReadyTickets ensures no internal blockers, but GetBlockers checks JQL-independent status.
		if len(blockers) > 0 {
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
		repoURL := extractRepoURL(description, jira.RepoRegex)

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

func extractRepoURL(text string, repoRegex *regexp.Regexp) string {
	if repoRegex == nil {
		return ""
	}
	matches := repoRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSuffix(matches[1], ".git")
	}
	return ""
}
