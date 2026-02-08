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

func TestMockAgent_Coding_InitializerPath(t *testing.T) {
	agent := NewMockAgent()
	// This prompt simulates the Coding Agent prompt when Initializer is used.
	// Task ID is "PRIMES" (no brackets in ID), and description is from Initializer response.
	prompt := `## ROLE: Coding Agent

You are a Senior Software Engineer.
Your task is to implement the feature described below.

### Task: PRIMES
Script calculates primes correctly

...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should trigger the Coding Agent heuristic (which returns "I will implement the prime number script...")
	if !strings.Contains(response, "I will implement the prime number script") {
		t.Errorf("Expected Coding Agent response, got: %s", response)
	}
}
