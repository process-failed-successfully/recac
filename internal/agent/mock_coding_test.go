package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_CodingHeuristic(t *testing.T) {
	agent := NewMockAgent()

	// Test Fallback
	prompt := "## YOUR ROLE - CODING AGENT\n\nTask: Unknown Task"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "```bash") {
		t.Errorf("Response should contain markdown code block, got: %s", resp)
	}

	if !strings.Contains(resp, "Coding agent fallback") {
		t.Errorf("Response should contain fallback message, got: %s", resp)
	}

	// Test Primes
	prompt = "## YOUR ROLE - CODING AGENT\n\nTask: req-primes-py-exists"
	resp, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "```bash") {
		t.Errorf("Response should contain markdown code block, got: %s", resp)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should generate primes.py, got: %s", resp)
	}
}
