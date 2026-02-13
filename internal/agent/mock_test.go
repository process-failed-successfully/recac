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

	// 1. Initializer Agent
	resp, err := agent.Send(ctx, "You are an Initializer Agent")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected initializer response, got: %s", resp)
	}

	// 2. TPM Agent
	resp, err = agent.Send(ctx, "You are a Technical Program Manager. Analyze the spec.")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected TPM response (ID:[PRIMES]), got: %s", resp)
	}
	if !strings.Contains(resp, "\"type\": \"Task\"") {
		t.Errorf("Expected JSON Task type, got: %s", resp)
	}

	// 3. Coding Agent (Primes)
	resp, err = agent.Send(ctx, "Please implement primes.py")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Expected python code, got: %s", resp)
	}
	if !strings.Contains(resp, "git commit") {
		t.Errorf("Expected git commit command, got: %s", resp)
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
