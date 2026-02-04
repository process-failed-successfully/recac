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

	// Mock agent now returns an implementation plan (bash script) by default for generic prompts
	if !strings.Contains(response, "I will implement the requested features") {
		t.Errorf("Response missing implementation text, got: %s", response)
	}

	if !strings.Contains(response, "COMPLETED") {
		t.Errorf("Response missing COMPLETED signal, got: %s", response)
	}
}

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Task: [PRIMES] Implement Prime Number Generator"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should contain the robust python script
	if !strings.Contains(response, "import json") {
		t.Error("Response for [PRIMES] should contain 'import json' in the python script")
	}
	if !strings.Contains(response, "json.dump(primes, f)") {
		t.Error("Response for [PRIMES] should contain json dumping logic")
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
