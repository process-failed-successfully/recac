package jira

import (
	"context"
	"fmt"
	"sync"
)

// MockClient simulates Jira API interactions for testing/mocking.
type MockClient struct {
	mu          sync.Mutex
	TicketCount int
}

// NewMockClient creates a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// CreateTicket simulates creating a ticket.
func (m *MockClient) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TicketCount++
	key := fmt.Sprintf("%s-%d", projectKey, m.TicketCount)
	fmt.Printf("[MockJira] Created Ticket %s: %s\n", key, summary)
	return key, nil
}

// CreateChildTicket simulates creating a child ticket.
func (m *MockClient) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TicketCount++
	key := fmt.Sprintf("%s-%d", projectKey, m.TicketCount)
	fmt.Printf("[MockJira] Created Child Ticket %s (Parent %s): %s\n", key, parentKey, summary)
	return key, nil
}

// AddIssueLink simulates linking issues.
func (m *MockClient) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	fmt.Printf("[MockJira] Linked %s -> %s (%s)\n", inwardKey, outwardKey, linkType)
	return nil
}
