package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Prompt simulating TPM Agent request for PRIMES scenario
	prompt := "You are the Technical Program Manager. ... ID:[PRIMES] ..."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected response to contain 'ID:[PRIMES]', got: %s", resp)
	}

	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected response to contain 'primes.py', got: %s", resp)
	}
}

func TestMockAgent_Send_TPM_Generic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Generic TPM prompt
	prompt := "You are the Technical Program Manager. Generate tickets."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected generic response NOT to contain 'ID:[PRIMES]', got: %s", resp)
	}

	if !strings.Contains(resp, "Implement Core Feature") {
		t.Errorf("Expected generic response to contain 'Implement Core Feature', got: %s", resp)
	}
}
