package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Ticket Generation", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager. Generate a ticket plan."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "```json") {
			t.Errorf("Expected JSON block in response, got: %s", resp)
		}
		if !strings.Contains(resp, "Calculate Primes") {
			t.Errorf("Expected 'Calculate Primes' in response, got: %s", resp)
		}
	})

	t.Run("Implementation - Primes", func(t *testing.T) {
		prompt := "Implement the code for [PRIMES]. Calculate Primes script."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "primes.py") {
			t.Errorf("Expected 'primes.py' in response, got: %s", resp)
		}
		if !strings.Contains(resp, "agent-bridge feature set") {
			t.Errorf("Expected agent-bridge command in response, got: %s", resp)
		}
	})

	t.Run("QA Agent", func(t *testing.T) {
		prompt := "YOUR ROLE - QA AGENT. Run tests."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA signal in response, got: %s", resp)
		}
	})

	t.Run("Project Manager Signoff", func(t *testing.T) {
		prompt := "You are the PROJECT MANAGER. Review status."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected Signoff signal in response, got: %s", resp)
		}
	})

	t.Run("Default Fallback", func(t *testing.T) {
		prompt := "Hello, who are you?"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected default mock response, got: %s", resp)
		}
	})
}
