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

func TestMockAgent_Prime_Response(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the prompt sent to the Agent for the Prime scenario
	// Use a minimal prompt that might occur if role header is missing/changed, but task is present.
	prompt := `...
### YOUR ASSIGNED TASK
- **Feature ID**: [PRIMES]
- **Description**: Create a python script named 'primes.py'...
...`

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify we got the bash implementation script, not the fallback
	if strings.HasPrefix(resp, "Mock agent response") {
		t.Errorf("Expected bash implementation script, got fallback response: %q", resp)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing file creation command")
	}

	if !strings.Contains(resp, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Errorf("Response missing completion signal")
	}
}

func TestMockAgent_Prime_Response_Keywords(t *testing.T) {
	agent := NewMockAgent()
	// Simulate prompt with just keywords if ID is missing (robustness check)
	prompt := `Task: Create primes.py that outputs json.`

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.HasPrefix(resp, "Mock agent response") {
		t.Errorf("Expected bash implementation script for keyword match, got fallback: %q", resp)
	}
}
