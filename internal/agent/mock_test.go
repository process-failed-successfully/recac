package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Fallback(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Random prompt"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expected := "I am a heuristic mock agent"
	if !strings.Contains(response, expected) {
		t.Errorf("Expected response to contain '%s', got: %s", expected, response)
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - TECHNICAL PROGRAM MANAGER: Generate tickets"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Error("Expected TPM response to contain ticket ID")
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - CODING AGENT: Please implement req-implement-prime-number-script"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "primes.py") {
		t.Error("Expected Coding response to contain primes.py creation")
	}
}
