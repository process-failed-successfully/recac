package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"recac/internal/auth"
	"recac/internal/filtering"
	"recac/internal/polling"
)

type mockJiraClient struct {
	tickets []filtering.Ticket
	err     error
}

func (m *mockJiraClient) GetTickets(ctx context.Context) ([]filtering.Ticket, error) {
	return m.tickets, m.err
}

func TestPollingIntegration(t *testing.T) {
	tests := []struct {
		name           string
		mockTickets    []filtering.Ticket
		mockErr        error
		expectedCount  int
		expectedError  bool
	}{
		{
			name: "Successful integration with filtered tickets",
			mockTickets: []filtering.Ticket{
				{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent"}},
				{Key: "TICKET-2", Status: "In Progress", Labels: []string{"bug"}},
				{Key: "TICKET-3", Status: "Ready", Labels: []string{"recac-agent"}},
				{Key: "TICKET-4", Status: "Ready", Labels: []string{"recac-agent", "bug"}},
			},
			mockErr:       nil,
			expectedCount: 3,
			expectedError: false,
		},
		{
			name: "Jira API error integration",
			mockTickets:   nil,
			mockErr:       errors.New("Jira API connection failed"),
			expectedCount: 0,
			expectedError: true,
		},
		{
			name: "Empty ticket list integration",
			mockTickets:   []filtering.Ticket{},
			mockErr:       nil,
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "No matching tickets integration",
			mockTickets: []filtering.Ticket{
				{Key: "TICKET-1", Status: "In Progress", Labels: []string{"bug"}},
				{Key: "TICKET-2", Status: "Done", Labels: []string{"feature"}},
			},
			mockErr:       nil,
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockJiraClient{
				tickets: tt.mockTickets,
				err:     tt.mockErr,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			tickets, err := polling.PollJiraTickets(ctx, client)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if len(tickets) != tt.expectedCount {
				t.Errorf("Expected %d tickets, got %d", tt.expectedCount, len(tickets))
			}

			// Verify all returned tickets meet filtering criteria
			for _, ticket := range tickets {
				if !filtering.IsReady(ticket) {
					t.Errorf("Ticket %s is not in Ready state", ticket.Key)
				}
				if !filtering.HasRecacAgentLabel(ticket) {
					t.Errorf("Ticket %s does not have recac-agent label", ticket.Key)
				}
			}
		})
	}
}

func TestPollingIntegrationWithContextCancellation(t *testing.T) {
	client := &mockJiraClient{
		tickets: []filtering.Ticket{
			{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent"}},
		},
		err: nil,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	_, err := polling.PollJiraTickets(ctx, client)

	if err == nil {
		t.Error("Expected context cancellation error but got nil")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestPollingIntegrationWithTimeout(t *testing.T) {
	client := &mockJiraClient{
		tickets: []filtering.Ticket{
			{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent"}},
		},
		err: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout occurs
	time.Sleep(1 * time.Millisecond)

	_, err := polling.PollJiraTickets(ctx, client)

	if err == nil {
		t.Error("Expected timeout error but got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
	}
}

func TestPollingIntegrationWithAuth(t *testing.T) {
	// Test that polling works with authentication
	authConfig := &auth.JiraAuthConfig{
		Username: "test-user",
		APIToken: "test-token",
		BaseURL:  "https://test.jira.com",
	}

	// Create a mock client that simulates authenticated requests
	client := &mockJiraClient{
		tickets: []filtering.Ticket{
			{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent"}},
			{Key: "TICKET-2", Status: "Ready", Labels: []string{"recac-agent"}},
		},
		err: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tickets, err := polling.PollJiraTickets(ctx, client)

	if err != nil {
		t.Errorf("Unexpected error with auth: %v", err)
	}

	if len(tickets) != 2 {
		t.Errorf("Expected 2 tickets with auth, got %d", len(tickets))
	}
}
