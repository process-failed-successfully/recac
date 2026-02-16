package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test TPM Heuristic
	tpmPrompt := "You are an expert Technical Program Manager (TPM)..."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") && !strings.Contains(resp, "Implement Prime Number Script") {
		t.Errorf("TPM response expected JSON with ID:[PRIMES], got: %s", resp)
	}
	if !strings.Contains(resp, "\"type\": \"Story\"") {
		t.Errorf("TPM response should contain JSON structure, got: %s", resp)
	}

	// Test Coding Agent Heuristic
	codingPrompt := "## YOUR ROLE - CODING AGENT\n..."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding response expected bash script, got: %s", resp)
	}

	// Test Default
	defaultPrompt := "Hello world"
	resp, err = agent.Send(ctx, defaultPrompt)
	if err != nil {
		t.Fatalf("Default Send failed: %v", err)
	}
	if !strings.Contains(resp, "I received your prompt") {
		t.Errorf("Default response expected generic ack, got: %s", resp)
	}
}
