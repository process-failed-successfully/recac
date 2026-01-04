package jira

import (
	"context"
)

// CreateTicket creates a new Jira ticket
func (c *Client) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, fields map[string]interface{}) (string, error) {
	issue := &Issue{
		Fields: &IssueFields{
			Project:     &Project{Key: projectKey},
			Summary:     summary,
			Description: description,
			Type:        &IssueType{Name: issueType},
		},
	}

	// Apply additional fields if provided
	if fields != nil {
		if issue.Fields.CustomFields == nil {
			issue.Fields.CustomFields = make(map[string]interface{})
		}
		for k, v := range fields {
			issue.Fields.CustomFields[k] = v
		}
	}

	newIssue, _, err := c.client.Issue.Create(issue)
	if err != nil {
		return "", err
	}

	return newIssue.Key, nil
}
