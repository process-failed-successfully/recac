package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Default fallback test
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

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Planning Heuristic (TPM + Prime/Python)
	// Should return JSON tickets, not python script
	prompt := "You are a Technical Program Manager. Generate tickets for a Prime Python script."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, `"type": "Epic"`) {
		t.Errorf("Expected JSON ticket output for TPM prompt, got: %s", resp)
	}
	if !strings.Contains(resp, `"title": "Prime Number Generator Epic"`) {
		t.Errorf("Expected Prime Epic title in JSON, got: %s", resp)
	}
	if strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Planning prompt should NOT return python code implementation")
	}

	// 2. Test Execution Heuristic (Prime + Python/Script, NO TPM)
	// Should return python script
	prompt = "Write a python script to calculate prime numbers."
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Expected python code implementation for Execution prompt, got: %s", resp)
	}
	if strings.Contains(resp, `"type": "Epic"`) {
		t.Errorf("Execution prompt should NOT return JSON tickets")
	}

	// 3. Test Initializer Heuristic
	prompt = "You are an Initializer Agent. Initialize the feature."
	resp, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import for Initializer prompt, got: %s", resp)
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
