package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"recac/internal/db"
	"recac/internal/jira"
	"regexp"
	"strings"
)

var (
	featuresHeaderRegex = regexp.MustCompile(`(?i)^(REQUIRED FEATURES|ACCEPTANCE CRITERIA):?\s*$`)
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
		return p.Client.GetBlockerKeys(issue)
	})

	// We only want items that are READY (no local blockers and no external blockers).
	// GetReadyTickets(nil) returns items with no internal dependencies in the current set.
	var curatedItems []WorkItem

	readyKeys := graph.GetReadyTickets(nil) // Empty completed set

	// Filter readyKeys for external blockers too (safe guard)
	finalKeys := make([]string, 0, len(readyKeys))
	seenKeys := make(map[string]bool)
	issueMap := make(map[string]map[string]interface{})
	for _, i := range issues {
		k, _ := i["key"].(string)
		issueMap[k] = i
	}

	for _, key := range readyKeys {
		if seenKeys[key] {
			continue
		}
		issue := issueMap[key]
		blockers := p.Client.GetBlockerKeys(issue)
		// If blockers exist (internal or external), skip.
		// GetReadyTickets ensures no internal blockers, but GetBlockerKeys checks JQL-independent status.
		if len(blockers) > 0 {
			continue
		}
		finalKeys = append(finalKeys, key)
		seenKeys[key] = true
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
		if repoURL == "" {
			// No repo URL found, skipping
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

		// Inject Required Features if present
		if features := extractRequiredFeatures(description); len(features) > 0 {
			fl := db.FeatureList{
				ProjectName: summary,
				Features:    features,
			}
			if data, err := json.Marshal(fl); err == nil {
				item.EnvVars["RECAC_INJECTED_FEATURES"] = string(data)
			}
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
		// Attempt transition using SmartTransition which handles ID or Name matching.
		return p.Client.SmartTransition(ctx, item.ID, status)
	}
	return nil
}

func (p *JiraPoller) Ping(ctx context.Context) error {
	// Simple query to verify connectivity
	jql := "created is not empty"
	if p.JQL != "" {
		jql = p.JQL
	}
	// We don't care about the results, just that the query succeeds
	_, err := p.Client.SearchIssues(ctx, jql)
	if err != nil {
		return fmt.Errorf("jira connectivity check failed: %w", err)
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

func extractRequiredFeatures(text string) []db.Feature {
	// Look for REQUIRED FEATURES: or ACCEPTANCE CRITERIA: block
	// Regex matches headers case-insensitively
	// Then captures lines starting with "- " or "* " until a blank line or new section
	var features []db.Feature

	lines := strings.Split(text, "\n")
	inSection := false

	// Optimized: uses package-level regex
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if featuresHeaderRegex.MatchString(line) {
			inSection = true
			continue
		}

		if inSection {
			if line == "" || strings.HasPrefix(line, "#") || (strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*")) {
				// End of section (empty line, comment, or new header)
				if line != "" {
					if strings.Contains(line, ":") {
						break
					}
				}
			}

			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				// Extract feature description
				desc := strings.TrimSpace(line[2:])
				// ⚡ Bolt: Replaced featureSlugRegex.ReplaceAllString and strings.ToLower
				// with a single-pass string builder to eliminate multiple intermediate allocations
				// and regex state machine overhead in high-throughput loops.
				var sb strings.Builder
				sb.Grow(len(desc))
				lastWasHyphen := false
				for i := 0; i < len(desc); i++ {
					c := desc[i]
					if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
						sb.WriteByte(c)
						lastWasHyphen = false
					} else if c >= 'A' && c <= 'Z' {
						sb.WriteByte(c + ('a' - 'A'))
						lastWasHyphen = false
					} else {
						if !lastWasHyphen && sb.Len() > 0 {
							sb.WriteByte('-')
							lastWasHyphen = true
						}
					}
				}

				slug := sb.String()
				slug = strings.Trim(slug, "-")
				if len(slug) > 30 {
					slug = slug[:30]
				}

				f := db.Feature{
					ID:          fmt.Sprintf("req-%s", slug),
					Description: desc,
					Category:    "functional",
					Priority:    "critical",
					Status:      "pending",
				}
				features = append(features, f)
			}
		}
	}
	return features
}
