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

func TestMockAgent_TPM_Precedence(t *testing.T) {
	agent := NewMockAgent()
	// Prompt that contains both "Technical Program Manager" (TPM trigger) and "prime" (Coding Agent trigger)
	prompt := "You are an expert Technical Program Manager (TPM). Spec: Implement prime number generator."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON list (TPM response), not python script (Coding Agent response)
	if !strings.Contains(response, `ID:[PRIMES] Implement Primes Script`) {
		t.Errorf("Expected TPM response (JSON), got: %s", response)
	}
	if strings.Contains(response, "import json") && strings.Contains(response, "def is_prime") {
		t.Errorf("Expected TPM response, got Python script (Coding Agent response)")
	}
}
