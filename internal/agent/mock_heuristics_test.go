package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgentHeuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("PrimesImplementation", func(t *testing.T) {
		prompt := "Please implement primes.py"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal COMPLETED true") {
			t.Error("Expected completion signal in primes implementation")
		}
	})

	t.Run("QAAgent", func(t *testing.T) {
		prompt := "YOUR ROLE - QA AGENT\nPlease verify."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Error("Expected QA_PASSED signal")
		}
	})

	t.Run("ManagerAgent", func(t *testing.T) {
		prompt := "YOUR ROLE - PROJECT MANAGER\nPlease review."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Error("Expected PROJECT_SIGNED_OFF signal")
		}
	})
}
