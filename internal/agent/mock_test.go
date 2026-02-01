package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("Default Response", func(t *testing.T) {
		prompt := "This is a test prompt that is long enough to be truncated"
		response, err := agent.Send(context.Background(), prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "Mock agent response") {
			t.Errorf("Response missing prefix, got: %s", response)
		}

		if !strings.Contains(response, "I received your prompt") {
			t.Errorf("Response missing body, got: %s", response)
		}
	})

	t.Run("Ticket Plan Generation", func(t *testing.T) {
		prompt := "Please generate-from-spec based on the requirements."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Expect JSON output
		if !strings.HasPrefix(strings.TrimSpace(response), "[") {
			t.Errorf("Expected JSON array response, got: %s", response)
		}
		if !strings.Contains(response, "PRIMES") {
			t.Errorf("Response missing PRIMES ticket ID, got: %s", response)
		}
	})

	t.Run("Implementation - Primes", func(t *testing.T) {
		prompt := "Implement the logic for [PRIMES] task."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "primes.py") {
			t.Errorf("Response should contain primes.py script generation, got: %s", response)
		}
		if !strings.Contains(response, "git commit") {
			t.Errorf("Response should contain git commit command, got: %s", response)
		}
	})
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
