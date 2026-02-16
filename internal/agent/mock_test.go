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

func TestMockAgent_E2E_Primes_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Simulate TPM Prompt
	prompt := "You are an expert Technical Program Manager (TPM)... please create a plan..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.HasPrefix(strings.TrimSpace(response), "[") {
		t.Errorf("Expected JSON array for TPM prompt, got: %s", response)
	}

	if !strings.Contains(response, "ID:[PRIMES] Implement prime number generator") {
		t.Errorf("Expected specific ticket title in plan, got: %s", response)
	}
}

func TestMockAgent_E2E_Primes_Coding(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Coding Prompt
	prompt := "Implement prime number generator"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return Bash script
	if !strings.Contains(response, "```bash") {
		t.Errorf("Expected bash block for coding prompt, got: %s", response)
	}

	if !strings.Contains(response, "primes.json") {
		t.Errorf("Expected primes.json output logic, got: %s", response)
	}

	if !strings.Contains(response, "range(10000)") {
		t.Errorf("Expected range(10000) for primes calculation, got: %s", response)
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
