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

func TestMockAgent_Heuristics_Planning(t *testing.T) {
	agent := NewMockAgent()
	prompt := "As a Technical Program Manager, generate tickets for the prime number spec."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "\"title\":") {
		t.Errorf("Response missing 'title' key, got: %s", resp)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("Response missing 'ID:[PRIMES]' tag, got: %s", resp)
	}
}

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	agent := NewMockAgent()
	// This prompt simulates an execution prompt that MIGHT contain TPM context
	prompt := "Context: You are working with a Technical Program Manager. Task: Implement ID:[PRIMES] prime number generator."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should match execution (python code) NOT planning (JSON)
	if strings.Contains(resp, "[") && strings.Contains(resp, "{") && strings.Contains(resp, "\"title\":") {
		t.Errorf("Matched Planning (JSON) instead of Execution. Response: %s", resp)
	}
	if !strings.Contains(resp, "filename: primes.py") {
		t.Errorf("Did not match Execution (Python code). Response: %s", resp)
	}
}

func TestMockAgent_Heuristics_PlanningWithPrimesFile(t *testing.T) {
	agent := NewMockAgent()
	// This prompt mimics the 'generate-from-spec' command which mentions TPM and also 'primes.py' (via spec)
	// It should return JSON (Planning), not Python (Execution).
	prompt := "As a Technical Program Manager, generate tickets. Spec: Output code to primes.py."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should match Planning (JSON)
	if !strings.Contains(resp, "\"title\":") {
		t.Errorf("Did not match Planning (JSON) when 'primes.py' is present in TPM request. Response: %s", resp)
	}
	if strings.Contains(resp, "filename: primes.py") {
		t.Errorf("Incorrectly matched Execution (Python code) for TPM request. Response: %s", resp)
	}
}
