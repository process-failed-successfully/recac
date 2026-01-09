package jira

import "context"

// ClientInterface defines the methods required by the Jira client for ticket generation.
type ClientInterface interface {
	CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error)
	CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error)
	AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error
}
