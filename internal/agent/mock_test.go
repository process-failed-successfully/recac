package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test 1: Default/Fallback Response
	t.Run("DefaultResponse", func(t *testing.T) {
		prompt := "This is a random prompt that should trigger fallback"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "Mock agent response") {
			t.Errorf("Response missing prefix, got: %s", response)
		}
		if !strings.Contains(response, "```bash") {
			t.Error("Fallback response missing bash block (circuit breaker safety)")
		}
	})

	// Test 2: TPM Heuristic
	t.Run("TPM_Heuristic", func(t *testing.T) {
		// Should match
		prompt := "You are the Technical Program Manager. Please generate tickets."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "\"type\": \"Story\"") {
			t.Error("TPM heuristic failed to match valid prompt")
		}

		// Should NOT match (False positive check)
		// e.g., Coding agent prompt mentions "tickets" but not "Technical Program Manager" role
		promptFalse := "I am the Coding Agent working on tickets."
		respFalse, _ := agent.Send(context.Background(), promptFalse)
		if strings.Contains(respFalse, "\"type\": \"Story\"") {
			t.Error("TPM heuristic falsely matched coding agent prompt")
		}
	})

	// Test 3: Implementation Heuristic (Primes)
	t.Run("Implementation_Heuristic", func(t *testing.T) {
		// Case insensitive check
		prompts := []string{
			"Calculate primes",
			"calculate primes",
			"Please implement a script to identify prime numbers", // "prime numbers"
			"Task: ID:[PRIMES] Implement...",
		}

		for _, p := range prompts {
			resp, err := agent.Send(context.Background(), p)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, "primes.py") {
				t.Errorf("Implementation heuristic failed for prompt: %q", p)
			}
			if !strings.Contains(resp, "agent-bridge feature set") {
				t.Errorf("Implementation response missing completion signal for prompt: %q", p)
			}
		}
	})

	// Test 4: Initializer
	t.Run("Initializer", func(t *testing.T) {
		prompt := "YOUR ROLE: INITIALIZER AGENT"
		resp, _ := agent.Send(context.Background(), prompt)
		if !strings.Contains(resp, "agent-bridge import") {
			t.Error("Initializer heuristic failed")
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
