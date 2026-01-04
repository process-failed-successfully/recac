package jira

import (
	"context"
	"fmt"
	"strings"
)

// AddComment adds a comment to a Jira issue
func (c *Client) AddComment(ctx context.Context, issueKey, comment string) error {
	_, err := c.client.Issue.AddComment(issueKey, &Comment{
		Body: comment,
	})
	return err
}

// ParseDescription parses a Jira issue description and returns structured data
func (c *Client) ParseDescription(ctx context.Context, issueKey string) (map[string]interface{}, error) {
	issue, _, err := c.client.Issue.Get(issueKey, nil)
	if err != nil {
		return nil, err
	}

	// Simple parsing - extract key-value pairs from description
	// Format: Key: Value
	lines := strings.Split(issue.Fields.Description, "\n")
	result := make(map[string]interface{})

	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}

	return result, nil
}

// GetBlockers returns the blocking issues for a given ticket
func (c *Client) GetBlockers(ctx context.Context, issueKey string) ([]string, error) {
	issue, _, err := c.client.Issue.Get(issueKey, nil)
	if err != nil {
		return nil, err
	}

	// Check if the issue has any blockers
	if issue.Fields.IssueLinks == nil {
		return []string{}, nil
	}

	var blockers []string
	for _, link := range issue.Fields.IssueLinks {
		if link.Type.Name == "Blocks" {
			blockers = append(blockers, link.OutwardIssue.Key)
		}
	}

	return blockers, nil
}
