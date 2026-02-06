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

func TestMockAgent_TPM_AmbiguousPrompt(t *testing.T) {
	agent := NewMockAgent()
	// This prompt simulates the TPM step in the E2E smoke test.
	// It contains "Technical Program Manager", but also "[PRIMES]" and "primes.py" (via spec).
	// It MUST be treated as a TPM prompt (return JSON), not a coding task (return Bash).
	prompt := `
You are the Technical Program Manager.
Here is the spec: app_spec.txt
ID:[PRIMES] Implement Primes Calculation
Please write a python script named primes.py
`
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected JSON list response (starting with '['), got:\n%s", resp)
	}
	if strings.Contains(resp, "I will implement") {
		t.Error("Response matched Coding Agent heuristic instead of TPM")
	}
}

func TestMockAgent_CodingAgent_FalsePositive(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a Coding Agent prompt that includes context about tickets AND the spec
	// The runner might include "app_spec.txt" in the context
	prompt := `
You are the Coding Agent.
Your task is to implement the feature described in the following tickets:
[
  {"title": "ID:[PRIMES] Implement Primes Calculation"}
]
Reference: app_spec.txt
Please write the code for primes.py.
`
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the Primes implementation (Bash script), NOT the JSON ticket list
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("FAIL: Got JSON response (TPM) instead of Coding Agent response. Response:\n%s", resp)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("FAIL: Expected bash script generation, got:\n%s", resp)
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
