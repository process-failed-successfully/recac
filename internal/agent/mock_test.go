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
	ctx := context.Background()

	// 1. Planning Heuristic
	planPrompt := "You are an expert Technical Program Manager (TPM)"
	planResp, err := agent.Send(ctx, planPrompt)
	if err != nil {
		t.Fatalf("Plan Send failed: %v", err)
	}
	if !strings.Contains(planResp, "ID:[PRIMES]") {
		t.Errorf("Expected plan response to contain ID:[PRIMES], got: %s", planResp)
	}

	// 2. Execution Heuristic
	execPrompt := "Please implement the prime number generator"
	execResp, err := agent.Send(ctx, execPrompt)
	if err != nil {
		t.Fatalf("Exec Send failed: %v", err)
	}
	if !strings.Contains(execResp, "def is_prime(n):") {
		t.Errorf("Expected exec response to contain python code, got: %s", execResp)
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
