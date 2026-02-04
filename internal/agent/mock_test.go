package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("Fallback Response", func(t *testing.T) {
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

	t.Run("Initializer Response", func(t *testing.T) {
		prompt := "Please initialize feature_list.json (Role: Initializer Agent)"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "```bash") {
			t.Error("Initializer response missing bash code block")
		}
		if !strings.Contains(response, "\"project_name\": \"Implement Primes\"") {
			t.Error("Initializer response missing project name")
		}
		if !strings.Contains(response, "\"id\": \"req-the-list-of-primes-in-primes-j\"") {
			t.Error("Initializer response missing feature ID")
		}
	})

	t.Run("Implementation Response", func(t *testing.T) {
		prompt := "Implement primes.py"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "```bash") {
			t.Error("Implementation response missing bash code block")
		}
		if !strings.Contains(response, "def calculate_primes") {
			t.Error("Implementation response missing python code")
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
