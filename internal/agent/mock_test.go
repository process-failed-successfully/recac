package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("Default Response", func(t *testing.T) {
		prompt := "This is a generic prompt"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "I received your prompt") {
			t.Errorf("Expected default response, got: %s", response)
		}
	})

	t.Run("TPM Heuristic", func(t *testing.T) {
		prompt := "You are a Technical Program Manager"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "PRIMES") || !strings.Contains(response, "json") {
			t.Errorf("Expected TPM JSON response, got: %s", response)
		}
	})

	t.Run("Primes Heuristic", func(t *testing.T) {
		prompt := "Please write a script to calculate prime numbers"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "primes.py") || !strings.Contains(response, "agent-bridge signal COMPLETED true") {
			t.Errorf("Expected Primes script response, got: %s", response)
		}
	})

	t.Run("QA Heuristic", func(t *testing.T) {
		prompt := "## YOUR ROLE - QA AGENT\n Verify the code."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA signal response, got: %s", response)
		}
	})

	t.Run("Manager Heuristic", func(t *testing.T) {
		prompt := "## YOUR ROLE - PROJECT MANAGER\n Review the project."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected Manager signal response, got: %s", response)
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
