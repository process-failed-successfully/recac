package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
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

func TestMockAgent_JSONResponse(t *testing.T) {
	agent := NewMockAgent()

	// Test Planning Prompt
	prompt := "You are an expert Technical Program Manager... ID:[PRIMES]..."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("expected JSON array response, got: %s", resp)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("expected response to contain task ID, got: %s", resp)
	}

	// Test Normal Prompt
	normalPrompt := "Hello agent"
	resp, err = agent.Send(context.Background(), normalPrompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("expected text response for normal prompt, got JSON: %s", resp)
	}
}
