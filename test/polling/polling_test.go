package polling_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"recac/internal/polling"
	"recac/internal/filtering"
)

type mockJiraClient struct {
	tickets []filtering.Ticket
	err     error
}

func (m *mockJiraClient) GetTickets(ctx context.Context) ([]filtering.Ticket, error) {
	return m.tickets, m.err
}

func TestPollJiraTickets(t *testing.T) {
	tests := []struct {
		name           string
		mockTickets    []filtering.Ticket
		mockErr        error
		expectedCalls  int
		expectedError  bool
		expectedCount  int
	}{
		{
			name: "Successful polling with filtered tickets",
			mockTickets: []filtering.Ticket{
				{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent"}},
				{Key: "TICKET-2", Status: "In Progress", Labels: []string{"bug"}},
				{Key: "TICKET-3", Status: "Ready", Labels: []string{"recac-agent"}},
			},
			mockErr:       nil,
			expectedCalls: 1,
			expectedError: false,
			expectedCount: 2,
		},
		{
			name: "Jira API error",
			mockTickets:   nil,
			mockErr:       errors.New("Jira API error"),
			expectedCalls: 1,
			expectedError: true,
			expectedCount: 0,
		},
		{
			name: "Empty ticket list",
			mockTickets:   []filtering.Ticket{},
			mockErr:       nil,
			expectedCalls: 1,
			expectedError: false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockJiraClient{
				tickets: tt.mockTickets,
				err:     tt.mockErr,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			tickets, err := polling.PollJiraTickets(ctx, client)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
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
				if !filtering.IsReady(ticket) || !filtering.HasRecacAgentLabel(ticket) {
					t.Errorf("Ticket %s does not meet filtering criteria", ticket.Key)
				}
			}
		})
	}
}

func TestPollJiraTicketsWithContextCancellation(t *testing.T) {
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

func TestPollJiraTicketsWithTimeout(t *testing.T) {
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
