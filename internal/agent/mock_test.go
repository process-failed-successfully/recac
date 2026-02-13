package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("Basic Response", func(t *testing.T) {
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

	t.Run("Initializer Trigger (Basic)", func(t *testing.T) {
		// Should return the bash script to create feature_list.json
		prompt := "You are an Initializer Agent. Please analyze the requirements."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "feature_list.json") {
			t.Errorf("Expected Initializer script, got: %s", response)
		}
	})

	t.Run("Initializer Trigger (Turn 1)", func(t *testing.T) {
		// Turn 1: Prompt contains "primes" (the task) and "feature_list.json" (instructions)
		// but NO output history ("agent-bridge import").
		// Should trigger Initializer Logic, NOT Coding Logic.
		prompt := "You are an Initializer Agent. Task: Implement primes.py. Output: feature_list.json."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Expect feature_list.json creation script
		if !strings.Contains(response, "cat << 'EOF' > feature_list.json") {
			t.Errorf("Expected Initializer script for Turn 1, got: %s", response)
		}

		// Should NOT return Python code yet
		if strings.Contains(response, "def get_primes(n):") {
			t.Errorf("Incorrectly returned Python code in Turn 1 (skipped initialization)")
		}
	})

	t.Run("Coding Agent Trigger with History (Turn 2+)", func(t *testing.T) {
		// Turn 2: Prompt contains "primes" (task) AND output history ("agent-bridge import").
		// Should trigger Coding Logic.
		prompt := `
User: You are an Initializer Agent.
Agent: cat << 'EOF' > feature_list.json ...
# Import the feature list using agent-bridge
cat feature_list.json | agent-bridge import
User: Great. Now act as a Coding Agent. Please implement the prime number script (primes.py).
`
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// We expect the Python script implementation
		if !strings.Contains(response, "def get_primes(n):") {
			t.Errorf("Expected Python implementation, got: %s", response)
		}

		// We do NOT expect the feature_list.json creation script again
		if strings.Contains(response, "cat << 'EOF' > feature_list.json") {
			t.Errorf("Incorrectly returned Initializer script again due to history match")
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
