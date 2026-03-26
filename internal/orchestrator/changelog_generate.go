package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// GenerateChangelog analyzes completed jobs and generates a Markdown changelog using an AI agent.
func GenerateChangelog(ctx context.Context, orch *Orchestrator, tag, match, provider, model, apiKey string) (string, error) {
	// Filter jobs
	var filtered []JobInfo
	jobs := orch.GetCompletedJobs()

	var matcher *regexp.Regexp
	if match != "" {
		m, err := regexp.Compile("(?i)" + match)
		if err != nil {
			return "", fmt.Errorf("invalid match regex: %v", err)
		}
		matcher = m
	}

	for _, job := range jobs {
		if job.Status != "Completed" {
			continue // double check just in case, though GetCompletedJobs should only return completed
		}

		if tag != "" {
			hasTag := false
			lowerTagFilter := strings.ToLower(tag)
			for _, t := range job.WorkItem.Tags {
				if strings.ToLower(t) == lowerTagFilter {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		if matcher != nil {
			if !matcher.MatchString(job.Summary) && !matcher.MatchString(job.Error) {
				continue
			}
		}

		filtered = append(filtered, job)
	}

	if len(filtered) == 0 {
		return "No completed jobs found matching the criteria.", nil
	}

	// Prepare job info string for AI context
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("I have %d completed jobs. Please generate a highly professional and clean Markdown changelog.\n", len(filtered)))
	sb.WriteString("Group them logically (e.g., Features, Bug Fixes, Maintenance/Chores). Add a short introductory paragraph.\n\n")

	for i, job := range filtered {
		// Include repo URL and summary, maybe part of description
		sb.WriteString(fmt.Sprintf("Job %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("ID: %s\n", job.ID))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", job.Summary))

		desc := job.WorkItem.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("Description snippet: %s\n\n", strings.ReplaceAll(desc, "\n", " ")))
	}

	// Get agent configuration
	if apiKey == "" {
		apiKey = viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
	}
	if provider == "" {
		provider = viper.GetString("orchestrator.agent_provider")
	}
	if model == "" {
		model = viper.GetString("orchestrator.agent_model")
	}

	aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	// Use context with timeout for AI call
	aiCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	explanation, err := aiClient.Send(aiCtx, sb.String())
	if err != nil {
		return "", fmt.Errorf("failed to get changelog from AI: %w", err)
	}

	return explanation, nil
}
