package orchestrator

import (
	"context"
	"recac/internal/jira"
)

// JiraClient defines the interface for the Jira client used by the poller.
type JiraClient interface {
	SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error)
	SmartTransition(ctx context.Context, ticketID, targetNameOrID string) error
	AddComment(ctx context.Context, ticketID, commentText string) error
	GetBlockers(ticket map[string]interface{}) []string
	ParseDescription(data map[string]interface{}) string
}

// Ensure jira.Client implements JiraClient interface.
var _ JiraClient = (*jira.Client)(nil)
