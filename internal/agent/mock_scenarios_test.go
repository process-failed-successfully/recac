package agent

import (
	"context"
	"recac/internal/agent/prompts"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM Ticket Generation
	// Construct a prompt that matches the template in internal/agent/prompts/templates/tpm_agent.md
	// plus the spec from pkg/e2e/scenarios/prime_python.go
	spec := "### ID:[PRIMES] Prime Number Script\nImplement a python script..."
	tpmPrompt, err := prompts.GetPrompt("tpm_agent", map[string]string{"spec": spec})
	if err != nil {
		t.Fatalf("failed to get tpm prompt: %v", err)
	}

	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected JSON response with ticket, got: %s", resp)
	}
	if strings.Contains(resp, "Mock agent response") {
		t.Errorf("Heuristic failed to trigger, got default response")
	}

	// 2. Coding Agent
	codingPrompt := "You are a Developer... Implement primes.py ... output to JSON..."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected coding response, got: %s", resp)
	}
}
