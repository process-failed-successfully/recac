package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_JSONResponse(t *testing.T) {
	agent := NewMockAgent()

	// Test case matching the smoke-test scenario
	prompt := "You are an expert Technical Program Manager... please create tickets... ID:[PRIMES]... output as JSON"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it returns valid JSON (starts with [)
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("expected JSON response starting with '[', got: %s", resp)
	}

	// Verify content
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("expected response to contain ticket ID, got: %s", resp)
	}
}

func TestMockAgent_DefaultResponse(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello world"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("expected default mock response, got: %s", resp)
	}
}
