package orchestrator

import (
	"context"
	"fmt"
	"io"
	"strings"

	"recac/internal/utils"
)

// GeneratePostmortem generates a markdown postmortem report from failed jobs.
func GeneratePostmortem(ctx context.Context, orch *Orchestrator, tag, match, provider, model, apiKey string) (string, error) {
	orch.mu.RLock()
	var failedJobs []JobInfo
	for _, job := range orch.completedJobs {
		if job.Status == "Failed" || job.Status == "error" {
			if match != "" && !utils.ContainsFold(job.Summary, match) && !utils.ContainsFold(job.Error, match) {
				continue
			}

			if tag != "" {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					// ⚡ Bolt: Use strings.EqualFold for zero-allocation case-insensitive comparisons
					if strings.EqualFold(t, tag) {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}
			failedJobs = append(failedJobs, job)
		}
	}
	orch.mu.RUnlock()

	if len(failedJobs) == 0 {
		return "No failed jobs found to generate a postmortem.", nil
	}

	// Limit to the most recent 10 failed jobs to avoid token limits
	if len(failedJobs) > 10 {
		failedJobs = failedJobs[len(failedJobs)-10:]
	}

	var sb strings.Builder
	sb.WriteString("Failed Jobs Context:\n")
	for _, job := range failedJobs {
		sb.WriteString(fmt.Sprintf("\n--- Job ID: %s ---\n", job.ID))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", job.Summary))
		sb.WriteString(fmt.Sprintf("Error: %s\n", job.Error))

		// Fetch logs
		logStream, err := orch.GetLogs(ctx, job.ID)
		var logsText string
		if err == nil && logStream != nil {
			logBytes, _ := io.ReadAll(logStream)
			logsText = string(logBytes)
			logStream.Close()
		}

		logLines := strings.Split(logsText, "\n")
		if len(logLines) > 100 {
			logLines = logLines[len(logLines)-100:]
			logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
		}

		sb.WriteString(fmt.Sprintf("Logs:\n```\n%s\n```\n", logsText))
	}

	aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert SRE and software engineer.
Write a comprehensive Postmortem Report for the following failed jobs in our autonomous coding orchestrator.

The report MUST be formatted in Markdown and MUST include:
1. **Executive Summary**: A high-level overview of the failures.
2. **Root Cause Analysis**: Group similar failures and explain the underlying technical reasons they failed.
3. **Action Items**: Concrete steps to fix the code, tests, or pipeline to prevent these failures in the future.

Here is the context of the failed jobs:
%s
`, sb.String())

	explanation, err := aiClient.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to get postmortem from AI: %w", err)
	}

	return explanation, nil
}
