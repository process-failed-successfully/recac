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

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please generate primes.py or id:[primes]"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("Expected primes.py script, got: %s", response)
	}
	if !strings.Contains(response, "is_prime") {
		t.Errorf("Expected is_prime function logic, got: %s", response)
	}
}

// Reproduction for Smoke Test Failure
// The ticket summary is "[PRIMES] Prime Number Script" and the prompt likely contains this.
// The heuristic "id:[primes]" requires "id:" which might be missing.
func TestMockAgent_Primes_SmokeTestRepro(t *testing.T) {
	agent := NewMockAgent()
	// Simulate prompt containing just the ticket summary
	prompt := "Task: [PRIMES] Prime Number Script\nDescription: Implement a Python script to check for prime numbers."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// This should fail with the current heuristic if it strictly requires "id:[primes]"
	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Logf("Response received: %s", response)
		t.Errorf("Expected primes.py script for '[PRIMES]' prompt, but got generic response")
	}
}
