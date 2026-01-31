package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_SmartMock(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Case 1: Jira Ticket Generation Prompt
	prompt := "You are an expert Technical Program Manager (TPM) with deep experience in agile software development. Create User Story and Epic..."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it returns valid JSON (heuristic check)
	if !strings.Contains(resp, "{") || !strings.Contains(resp, "PRIMES") {
		t.Errorf("Expected JSON response containing 'PRIMES', got: %s", resp)
	}

	// Test Case 2: Generic Prompt
	prompt = "Hello world"
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected generic mock response, got: %s", resp)
	}
}
