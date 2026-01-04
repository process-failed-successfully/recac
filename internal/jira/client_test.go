package jira

import (
	"testing"
)

func TestJiraClient(t *testing.T) {
	// This is a basic test structure
	// In a real scenario, you would mock the HTTP client or use a test Jira instance

	t.Run("Test NewClient", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		if client == nil {
			t.Fatal("Client is nil")
		}

		if client.BaseURL == "" {
			t.Error("BaseURL is empty")
		}
	})

	// Note: Actual API tests would require a test Jira instance
	// and proper mocking of HTTP responses
}
