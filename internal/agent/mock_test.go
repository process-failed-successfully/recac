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

	// 1. Test Completion Signal (Highest Priority)
	completionPrompt := "On branch agent/123\nYour branch is up to date.\nnothing to commit, working tree clean\nEverything up-to-date\n"
	compResp, _ := agent.Send(context.Background(), completionPrompt)
	if !strings.Contains(compResp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected completion signal for 'nothing to commit', got: %s", compResp)
	}

	// 2. Test Ticket Planning
	planPrompt := "You are a Technical Program Manager. Please generate ticket..."
	planResp, _ := agent.Send(context.Background(), planPrompt)
	if !strings.Contains(planResp, "\"id\": \"[PRIMES]\"") {
		t.Errorf("Expected JSON ticket list for planning prompt, got: %s", planResp)
	}

	// 3. Test Execution (Prime Python)
	execPrompt := "Write a python script to calculate prime numbers."
	execResp, _ := agent.Send(context.Background(), execPrompt)
	if !strings.Contains(execResp, "def is_prime(n):") {
		t.Errorf("Expected python code for execution prompt, got: %s", execResp)
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
