package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: Prompt with ticket ID
	prompt1 := "Context: You are a coding agent working on ticket MFLP-11017: Implement primes.py."
	resp1, err := agent.Send(ctx, prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes implementation for prompt '%s', got: %s", prompt1, resp1)
	}

	// Test case: Prompt with 'primes.py'
	prompt2 := "Please write a script named primes.py to calculate prime numbers."
	resp2, err := agent.Send(ctx, prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes implementation for prompt '%s', got: %s", prompt2, resp2)
	}
}

func TestMockAgent_Send_TPM_Strict(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: Prompt mentioning TPM but NOT as the role
	prompt1 := "The ticket was created by a TPM yesterday. Please implement the code."
	resp1, err := agent.Send(ctx, prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// Should NOT match TPM role (which returns JSON), should match Fallback or Coding Agent if "code" triggers it (but "code" doesn't).
	// It should definitely NOT contain the TPM JSON structure.
	if strings.Contains(resp1, "\"type\": \"Epic\"") {
		t.Errorf("Prompt '%s' incorrectly triggered TPM role response.", prompt1)
	}

	// Test case: Prompt explicitly defining TPM role
	prompt2 := "You are an expert Technical Program Manager. Create a plan for the project."
	resp2, err := agent.Send(ctx, prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "\"type\": \"Epic\"") {
		t.Errorf("Expected TPM JSON response for prompt '%s', got: %s", prompt2, resp2)
	}
}
