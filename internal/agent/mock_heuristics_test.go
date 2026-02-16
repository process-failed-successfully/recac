package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Heuristic", func(t *testing.T) {
		prompt := "You are the Technical Program Manager. Generate jira tickets."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, `"id": "PRIMES"`) {
			t.Errorf("Expected JSON ticket list, got: %s", resp)
		}
	})

	t.Run("Manager Heuristic", func(t *testing.T) {
		prompt := "Here is the QA Report. Please sign off as Project Manager."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF") {
			t.Errorf("Expected sign-off command, got: %s", resp)
		}
	})

	t.Run("Primes Coding Heuristic", func(t *testing.T) {
		prompt := "Implement the prime number checker."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "def is_prime(n):") {
			t.Errorf("Expected python code, got: %s", resp)
		}
	})
}
