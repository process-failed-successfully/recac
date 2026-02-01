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

func TestMockAgent_PrimesImplementation(t *testing.T) {
	agent := NewMockAgent()

	// Test case 1: Standard primes.py prompt
	prompt1 := "Implement primes.py to calculate prime numbers"
	resp1, err := agent.Send(context.Background(), prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "Create primes.py") {
		t.Errorf("Response missing implementation plan, got: %s", resp1)
	}
	if !strings.Contains(resp1, "agent-bridge feature set PRIMES") {
		t.Errorf("Response missing PRIMES completion, got: %s", resp1)
	}

	// Test case 2: Feature ID extraction
	prompt2 := "Feature ID: req-implement-prime-calculation-lo\nDescription: Implement prime calculation logic in primes.py"
	resp2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "agent-bridge feature set req-implement-prime-calculation-lo") {
		t.Errorf("Response did not update specific feature ID, got: %s", resp2)
	}

	// Test case 3: JSON file trigger
	prompt3 := "Feature ID: req-json\nDescription: Output results to primes.json"
	resp3, err := agent.Send(context.Background(), prompt3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp3, "Create primes.py") {
		t.Errorf("Response missing implementation plan for JSON trigger, got: %s", resp3)
	}
	if !strings.Contains(resp3, "agent-bridge feature set req-json") {
		t.Errorf("Response did not update specific feature ID for JSON trigger, got: %s", resp3)
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
