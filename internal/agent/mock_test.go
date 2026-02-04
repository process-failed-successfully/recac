package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TPM_Response(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)..." // Minimal prompt to trigger detection

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify response is JSON (starts with [) and not the default message
	if strings.HasPrefix(resp, "Mock agent response") {
		t.Errorf("Expected JSON response for TPM prompt, got default text response: %q", resp)
	}

	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected JSON array starting with '[', got: %q", resp)
	}

	if !strings.Contains(resp, "Implement Prime Number Generator") {
		t.Errorf("Expected response to contain 'Implement Prime Number Generator', got: %q", resp)
	}
}

func TestMockAgent_Default_Response(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello world"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.HasPrefix(resp, "Mock agent response") {
		t.Errorf("Expected default text response, got: %q", resp)
	}
}
