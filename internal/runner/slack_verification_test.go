package runner

import (
	"context"
	"recac/internal/telemetry"
	"testing"
)

// SpyNotifier captures notification calls for verification
type SpyNotifier struct {
	Reactions []struct {
		Timestamp string
		Reaction  string
	}
	Messages []struct {
		EventType string
		Message   string
		ThreadTS  string
	}
}

func (s *SpyNotifier) Start(ctx context.Context) {}

func (s *SpyNotifier) Notify(ctx context.Context, eventType, message, threadTS string) (string, error) {
	s.Messages = append(s.Messages, struct {
		EventType string
		Message   string
		ThreadTS  string
	}{EventType: eventType, Message: message, ThreadTS: threadTS})
	return "mock-ts", nil
}

func (s *SpyNotifier) AddReaction(ctx context.Context, timestamp, reaction string) error {
	s.Reactions = append(s.Reactions, struct {
		Timestamp string
		Reaction  string
	}{Timestamp: timestamp, Reaction: reaction})
	return nil
}

// MockJiraClient for verification
type MockJiraClient struct{}

func (m *MockJiraClient) AddComment(ctx context.Context, ticketID, comment string) error {
	return nil
}

func (m *MockJiraClient) SmartTransition(ctx context.Context, ticketID, target string) error {
	return nil
}

func TestCompleteJiraTicket_AddsCheckmark(t *testing.T) {
	spy := &SpyNotifier{}

	session := &Session{
		Project:       "TEST-PROJ",
		Notifier:      spy,
		JiraClient:    &MockJiraClient{},
		JiraTicketID:  "TEST-123",
		RepoURL:       "http://github.com/example/repo",
		SlackThreadTS: "initial-thread-ts",
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// Execute the private method
	session.completeJiraTicket(context.Background(), "http://github.com/example/repo/commit/sha")

	// Verify Checkmark
	foundCheckmark := false
	for _, r := range spy.Reactions {
		if r.Reaction == "white_check_mark" {
			foundCheckmark = true
			if r.Timestamp != "initial-thread-ts" {
				t.Errorf("Expected reaction on thread TS 'initial-thread-ts', got '%s'", r.Timestamp)
			}
		}
	}

	if !foundCheckmark {
		t.Error("Expected :white_check_mark: reaction, but none found")
	}

	// Verify Notification Message contains links
	foundMsg := false
	for _, m := range spy.Messages {
		if m.EventType == "on_project_complete" {
			foundMsg = true
			// Check content if needed, but primary goal is checkmark
		}
	}
	if !foundMsg {
		t.Error("Expected completion notification message, but none found")
	}
}
