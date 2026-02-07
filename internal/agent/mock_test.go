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

		if !strings.Contains(response, "```bash") {
			t.Errorf("Response missing bash block, got: %s", response)
		}
	})

	t.Run("Initializer Response", func(t *testing.T) {
		prompt := "You are the Initializer agent."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "feature_list.json") {
			t.Errorf("Response missing feature_list.json creation, got: %s", response)
		}
	})

	t.Run("TPM Response", func(t *testing.T) {
		prompt := "You are the TPM."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "\"type\": \"Epic\"") {
			t.Errorf("Response missing JSON plan, got: %s", response)
		}
		if !strings.Contains(response, "Implement Prime Number Script") {
			t.Errorf("Response missing Primes task, got: %s", response)
		}
	})

	t.Run("Coding Agent Primes Response", func(t *testing.T) {
		// Simulates a prompt from the Coding Agent prompt template which contains "feature_list.json"
		prompt := `## YOUR ROLE - CODING AGENT
		...
		cat feature_list.json | head -50
		...
		Task: Create a python script named primes.py`

		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Should NOT return the Initializer response (which creates feature_list.json)
		if strings.Contains(response, "Mock Initializer: Creating feature list") {
			t.Errorf("Incorrectly triggered Initializer response for Coding Agent prompt")
		}

		// Should return the Primes implementation
		if !strings.Contains(response, "Implementing prime number script") {
			t.Errorf("Failed to trigger Primes implementation response, got: %s", response)
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
