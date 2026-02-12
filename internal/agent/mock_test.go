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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test TPM
	tpmPrompt := "You are an expert Technical Program Manager. ID:[PRIMES] Scenario."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES] Implement Prime Number Generator") {
		t.Errorf("TPM heuristic failed, got: %s", resp)
	}

	// Test Initializer
	initPrompt := "You are an Initializer Agent."
	resp, err = agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Initializer heuristic failed, got: %s", resp)
	}

	// Test Coding Agent
	codePrompt := "Write a python script primes.py for prime number generation."
	resp, err = agent.Send(ctx, codePrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding heuristic failed, got: %s", resp)
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
