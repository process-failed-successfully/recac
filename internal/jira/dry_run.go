package jira

import (
	"context"
	"fmt"
	"sync/atomic"
)

// DryRunClient implements ClientInterface for dry-run operations.
// It prints what would happen instead of interacting with the Jira API.
type DryRunClient struct {
	counter int64
}

// NewDryRunClient creates a new DryRunClient.
func NewDryRunClient() *DryRunClient {
	return &DryRunClient{}
}

func (c *DryRunClient) nextID() string {
	id := atomic.AddInt64(&c.counter, 1)
	return fmt.Sprintf("DRY-%d", id)
}

// CreateTicket simulates creating a ticket.
func (c *DryRunClient) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	key := c.nextID()
	fmt.Printf("[DRY-RUN] Creating %s in project %s\n", issueType, projectKey)
	fmt.Printf("  Key: %s\n", key)
	fmt.Printf("  Summary: %s\n", summary)
	fmt.Printf("  Labels: %v\n", labels)
	// Truncate description for readability
	desc := description
	if len(desc) > 100 {
		desc = desc[:97] + "..."
	}
	fmt.Printf("  Description: %s\n", desc)
	return key, nil
}

// CreateChildTicket simulates creating a child ticket.
func (c *DryRunClient) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	key := c.nextID()
	fmt.Printf("[DRY-RUN] Creating Child %s under %s in project %s\n", issueType, parentKey, projectKey)
	fmt.Printf("  Key: %s\n", key)
	fmt.Printf("  Summary: %s\n", summary)
	fmt.Printf("  Labels: %v\n", labels)
	desc := description
	if len(desc) > 100 {
		desc = desc[:97] + "..."
	}
	fmt.Printf("  Description: %s\n", desc)
	return key, nil
}

// AddIssueLink simulates linking issues.
func (c *DryRunClient) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	fmt.Printf("[DRY-RUN] Linking %s -> %s (%s)\n", inwardKey, outwardKey, linkType)
	return nil
}
