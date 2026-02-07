package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	// Use a prompt that matches the heuristic (Technical Program Manager AND Specification/Plan/Tickets)
	prompt := "You are an expert Technical Program Manager (TPM) with deep experience... \n### Application Specification:"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return JSON, not "Mock agent response"
	if strings.HasPrefix(resp, "Mock agent response") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", resp)
	}

	if !strings.Contains(resp, "Implement Primes Calculation") {
		t.Errorf("Expected ticket JSON, got: %s", resp)
	}

	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Expected ticket ID tag, got: %s", resp)
	}
}
