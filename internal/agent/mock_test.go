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

func TestMockAgent_SmartPrimes(t *testing.T) {
	agent := NewMockAgent()

	// 1. Coding Task Prompt
	codingPrompts := []string{
		"Please implement the [PRIMES] task.",
		"Implement prime-python scenario.",
		"Create a Prime Number Script",
	}

	for _, p := range codingPrompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Errorf("Send failed for prompt '%s': %v", p, err)
		}

		if !strings.Contains(resp, "primes.py") {
			t.Errorf("Response for '%s' should contain primes.py", p)
		}
		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Response for '%s' should contain bash heredoc", p)
		}
	}

	// 2. TPM / Planning Prompt
	tpmPrompts := []string{
		"You are the Technical Program Manager. CRITICAL INSTRUCTION FOR TICKET GENERATION: Create exactly ONE ticket.",
		"Role: Technical Program Manager. ID:[PRIMES]. Generate ticket.",
	}

	for _, p := range tpmPrompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Errorf("Send failed for prompt '%s': %v", p, err)
		}

		// Should return JSON
		if !strings.Contains(resp, "[") || !strings.Contains(resp, "{") {
			t.Errorf("Response for '%s' should appear to be JSON array", p)
		}
		if !strings.Contains(resp, "\"id\": \"PRIMES\"") {
			t.Errorf("Response for '%s' should contain PRIMES ID in JSON", p)
		}
		// Should NOT return python code
		if strings.Contains(resp, "import json") {
			t.Errorf("Response for '%s' should NOT contain python code", p)
		}
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
