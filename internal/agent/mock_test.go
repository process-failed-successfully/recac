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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM Role (Planning)
	tpmPrompt := "You are an expert Technical Program Manager. Please create tickets for a prime number script."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Send (TPM) failed: %v", err)
	}

	// Should return JSON
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("TPM response should start with '[', got: %s", resp)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM response should contain ticket ID, got: %s", resp)
	}

	// 2. Coding Agent Role (Implementation)
	codingPrompt := "You are an expert software engineer. Write a python script for primes.py."
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Send (Coding) failed: %v", err)
	}

	// Should return Bash Script
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding response should contain bash script to create file, got: %s", resp)
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Coding response should NOT be JSON list, got: %s", resp)
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
