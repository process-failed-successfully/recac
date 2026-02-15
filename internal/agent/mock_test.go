package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test Default Response
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

	// Test TPM Heuristic
	tpmPrompt := "As a Technical Program Manager, please generate a ticket plan in JSON format."
	tpmResponse, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(tpmResponse, "ID:[SYSTEM]") {
		t.Errorf("TPM response missing expected ID, got: %s", tpmResponse)
	}
	if !strings.Contains(tpmResponse, "```json") {
		t.Errorf("TPM response missing json block, got: %s", tpmResponse)
	}

	// Test Coding Heuristic
	codingPrompt := "Write a python script to generate prime numbers."
	codingResponse, err := agent.Send(context.Background(), codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(codingResponse, "primes.py") {
		t.Errorf("Coding response missing filename, got: %s", codingResponse)
	}
	if !strings.Contains(codingResponse, "```bash") {
		t.Errorf("Coding response missing bash block, got: %s", codingResponse)
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
