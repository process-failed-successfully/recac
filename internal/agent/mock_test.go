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

	// 1. Test Ticket Planning
	planPrompt := "You are a Technical Program Manager. Please generate ticket..."
	planResp, _ := agent.Send(context.Background(), planPrompt)
	if !strings.Contains(planResp, "\"id\": \"[PRIMES]\"") {
		t.Errorf("Expected JSON ticket list for planning prompt, got: %s", planResp)
	}

	// 2. Test Execution (Prime Python)
	execPrompt := "Write a python script to calculate prime numbers."
	execResp, _ := agent.Send(context.Background(), execPrompt)
	if !strings.Contains(execResp, "def is_prime(n):") {
		t.Errorf("Expected python code for execution prompt, got: %s", execResp)
	}

	// 3. Test Loop Prevention (Already Committed)
	loopPrompt := "Write a python script to calculate prime numbers.\n\nCommand Output:\nOn branch agent/MFLP-12048\nnothing to commit, working tree clean\nEverything up-to-date"
	loopResp, _ := agent.Send(context.Background(), loopPrompt)
	if strings.Contains(loopResp, "def is_prime(n):") {
		t.Errorf("Agent should NOT return code again if work is already committed. Got code block: %s", loopResp)
	}
	if !strings.Contains(loopResp, "agent-bridge feature set") {
		t.Errorf("Agent should return command to mark feature as done. Got: %s", loopResp)
	}
}

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	agent := NewMockAgent()

	// Prompt contains BOTH "Initializer Agent" and "prime python"
	// It should trigger Initializer logic, NOT Prime logic
	mixedPrompt := "You are the Initializer Agent. Please read the spec: Create a python script named primes.py..."

	resp, _ := agent.Send(context.Background(), mixedPrompt)

	if strings.Contains(resp, "def is_prime(n):") {
		t.Errorf("Heuristic Priority Failed: Got python code instead of Initializer JSON. Response: %s", resp)
	}

	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected Initializer JSON response, got: %s", resp)
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
