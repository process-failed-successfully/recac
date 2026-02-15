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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	// Test PRIMES heuristic
	prompt := "Please implement the task ID:[PRIMES] Create Prime Number Script"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected python script in response, got: %s", resp)
	}

	// Test QA heuristic
	prompt = "Please perform QA on the changes"
	resp, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "QA_PASSED") {
		t.Errorf("Expected QA_PASSED signal, got: %s", resp)
	}
}

func TestMockAgent_Heuristics_Initializer_Priority(t *testing.T) {
	agent := NewMockAgent()

	// Simulating the exact prompt structure that caused the failure
	// It contains "ID:[PRIMES]" but also "Create a SINGLE Ticket"
	prompt := "### ID:[PRIMES] Prime Number Script\n\nCRITICAL INSTRUCTION: You MUST create exactly ONE ticket."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON list of tickets, NOT the bash script
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected JSON response for Initializer prompt, got: %s", resp)
	}
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Incorrectly returned coding script for Initializer prompt")
	}
}
