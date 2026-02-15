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

	// TPM Heuristic
	tpmPrompt := "You are an expert Technical Program Manager..."
	tpmResp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(tpmResp, "ID:[PRIMES]") {
		t.Errorf("TPM response missing ID:[PRIMES], got: %s", tpmResp)
	}
	if !strings.Contains(tpmResp, "[") || !strings.Contains(tpmResp, "]") {
		t.Errorf("TPM response does not look like JSON list, got: %s", tpmResp)
	}

	// Coding Heuristic
	codePrompt := "Implement a Python script to generate primes"
	codeResp, err := agent.Send(context.Background(), codePrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(codeResp, "def generate_primes(n):") {
		t.Errorf("Coding response missing python function, got: %s", codeResp)
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
