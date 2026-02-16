package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Case 1: Planning Phase (TPM)
	// This prompt should trigger the JSON ticket list response
	tpmPrompt := "You are an expert Technical Program Manager (TPM). Create a ticket plan for implementing a prime number generator."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(resp, `"tickets": [`) {
		t.Errorf("Expected JSON ticket list for TPM prompt, got: %s", resp)
	}
	if !strings.Contains(resp, "Implement prime number generator") {
		t.Errorf("Expected specific ticket summary, got: %s", resp)
	}

	// Test Case 2: Coding Task (Primes)
	// This prompt should trigger the Python code response
	codingPrompt := "Implement a prime number generator in python. Save it as primes.py"
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Expected Python code for prime generator, got: %s", resp)
	}
	if !strings.Contains(resp, "```python") {
		t.Errorf("Expected markdown code block, got: %s", resp)
	}

	// Test Case 3: Generic Prompt
	// This should return the standard mock response
	genericPrompt := "Hello, how are you today?"
	resp, err = agent.Send(ctx, genericPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response:") {
		t.Errorf("Expected generic mock response, got: %s", resp)
	}
	if strings.Contains(resp, `"tickets": [`) {
		t.Errorf("Did not expect JSON tickets for generic prompt, got: %s", resp)
	}
}
