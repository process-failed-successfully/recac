package jira

import (
	"context"
	"fmt"
)

// GetBlockers returns the blocking issues for a given ticket
func (c *Client) GetBlockers(ticket map[string]interface{}) ([]string, error) {
	// Extract issue key from ticket map
	issueKey, ok := ticket["key"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid ticket format: missing key")
	}

	return c.GetBlockersByKey(context.Background(), issueKey)
}

// GetBlockersByKey returns the blocking issues for a given issue key
func (c *Client) GetBlockersByKey(ctx context.Context, issueKey string) ([]string, error) {
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
