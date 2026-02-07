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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()

	// 1. Initializer Heuristic
	initPrompt := "## YOUR ROLE - INITIALIZER AGENT\nSpec: [PRIMES] Prime Number Script"
	resp, err := agent.Send(context.Background(), initPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected Initializer heuristic to trigger, got: %s", resp)
	}

	// 2. Coding Heuristic
	codingPrompt := "## YOUR ROLE - CODING AGENT\nTask: req-primes-implementation"
	resp, err = agent.Send(context.Background(), codingPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "primes.py") || !strings.Contains(resp, "primes.json") {
		t.Errorf("Expected Coding heuristic to trigger, got: %s", resp)
	}

	// 3. Manager Heuristic
	managerPrompt := "## YOUR ROLE - MANAGER AGENT\nQA Report: ..."
	resp, err = agent.Send(context.Background(), managerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected Manager heuristic to trigger, got: %s", resp)
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
