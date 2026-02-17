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
	featureSlugRegex    = regexp.MustCompile("[^a-z0-9]+")
)

type JiraPoller struct {
	Client JiraClient
	JQL    string
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

	// We only want items that are READY (no blockers).
	var curatedItems []WorkItem

	for _, issue := range issues {
		key, _ := issue["key"].(string)

		// Check for blockers (internal or external).
		// GetBlockerKeys returns keys of issues that block this one and are NOT in a Done state.
		blockers := p.Client.GetBlockerKeys(issue)
		if len(blockers) > 0 {
			continue
		}

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
				// Create a simplified Feature
				slug := strings.ToLower(desc)

				// Optimized: uses package-level regex
				slug = featureSlugRegex.ReplaceAllString(slug, "-")
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
