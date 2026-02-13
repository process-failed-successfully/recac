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

	// Test TPM Heuristic
	tpmPrompt := "As a Technical Program Manager, generate ticket for primes.py"
	resp, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM heuristic failed, missing ID:[PRIMES]")
	}

	// Test Primes Execution Heuristic
	execPrompt := "Create a script for prime numbers"
	resp, err = agent.Send(context.Background(), execPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "```bash") {
		t.Errorf("Primes heuristic failed, missing bash block")
	}
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Primes heuristic failed, missing file creation")
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
