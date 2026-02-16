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

func TestMockAgent_PrimesHeuristic(t *testing.T) {
	agent := NewMockAgent()

	// Test 1: Implementation
	prompt := "Implement Prime Number Generator"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Errorf("Response missing bash block marker, got: %s", response)
	}

	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("Response missing primes.py implementation, got: %s", response)
	}

	// Test 2: Tests
	prompt = "Write Unit Tests for Primes"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Errorf("Response missing bash block marker for tests prompt, got: %s", response)
	}

	if !strings.Contains(response, "cat <<EOF > test_primes.py") {
		t.Errorf("Response missing test_primes.py implementation for tests prompt, got: %s", response)
	}

	// Test 3: Success
	prompt = "OK\nRan 2 tests in 0.000s"
	// Ensure context contains "prime" for the heuristic to trigger (simulating conversation history or prompt injection)
	// The heuristic in mock.go checks prompt for "prime".
	// In real agent loop, prompt includes previous messages.
	// For this unit test, we just append "prime" to the prompt to trigger the heuristic.
	prompt = "Primes test output:\n" + prompt

	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge feature set") {
		t.Errorf("Response missing agent-bridge command for success prompt, got: %s", response)
	}

	if !strings.Contains(response, "|| true") {
		t.Errorf("Response missing || true suffix for robustness, got: %s", response)
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
