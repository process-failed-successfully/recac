package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test default response
	resp, err := agent.Send(ctx, "Hello")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected default response, got: %s", resp)
	}

	// Test TPM response logic
	tpmPrompt := "You are an expert Technical Program Manager (TPM) with deep experience in agile software development and technical systems design.\n\nYour task is to analyze..."
	resp, err = agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(resp, "```json") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", resp)
	}

	// Test Initializer response logic
	initPrompt := "## YOUR ROLE - INITIALIZER AGENT\n\nYour job is to set up..."
	resp, err = agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import for Initializer prompt, got: %s", resp)
	}

	// Test forced response
	agent.SetResponse("Forced")
	resp, err = agent.Send(ctx, "Hello")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp != "Forced" {
		t.Errorf("Expected 'Forced', got: %s", resp)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello"
	if truncateString(s, 10) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 10))
	}

	long := "hello world"
	if truncateString(long, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(long, 5))
	}
}
