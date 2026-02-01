package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Basics(t *testing.T) {
	agent := NewMockAgent()

	// Default
	prompt := "Hello"
	resp, _ := agent.Send(context.Background(), prompt)
	if !strings.Contains(resp, "no-op") {
		t.Errorf("Expected no-op for default prompt, got: %s", resp)
	}

	// Truncation
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
}

func TestMockAgent_Scenarios(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer
	resp, _ := agent.Send(ctx, "Please Initialize the project")
	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("Expected feature_list.json in initializer response, got: %s", resp)
	}

	// 2. Primes
	resp, _ = agent.Send(ctx, "Please implement req-primes")
	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Expected primes.py in primes response, got: %s", resp)
	}
	if !strings.Contains(resp, "python3 primes.py") {
		t.Errorf("Expected execution of primes.py, got: %s", resp)
	}

	// 3. QA
	resp, _ = agent.Send(ctx, "You are the QA AGENT")
	if !strings.Contains(resp, "QA_PASSED") {
		t.Errorf("Expected QA_PASSED signal, got: %s", resp)
	}

	// 4. Manager
	resp, _ = agent.Send(ctx, "You are the PROJECT MANAGER")
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
	}
}

func TestMockAgent_Completion(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	resp, _ := agent.Send(ctx, "req-primes: git status says nothing to commit")
	if !strings.Contains(resp, "agent-bridge signal COMPLETED") {
		t.Errorf("Expected completion signal, got: %s", resp)
	}
}

func TestMockAgent_GenerateFromSpec(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	resp, _ := agent.Send(ctx, "recac generate-from-spec")
	if !strings.Contains(resp, `"ID": "PRIMES"`) {
		t.Errorf("Expected JSON ticket list, got: %s", resp)
	}
}
