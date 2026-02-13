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

	// Scenario 1: Initial Coding Prompt
	prompt := "Write a python script to calculate primes."
	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Expected primes.py script, got: %s", response)
	}

	// Scenario 2: Coding Completion (Already Committed)
	prompt = "Write a python script to calculate primes.\n\n... nothing to commit, working tree clean"
	response, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal, got: %s", response)
	}

	// Scenario 3: Coding Completion (Up-to-date)
	prompt = "Write a python script to calculate primes.\n\n... Everything up-to-date"
	response, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal, got: %s", response)
	}

	// Scenario 4: QA Agent
	prompt = "## YOUR ROLE - QA AGENT\n\nVerify the project."
	response, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA PASSED signal, got: %s", response)
	}

	// Scenario 5: Manager Agent
	prompt = "## YOUR ROLE - PROJECT MANAGER\n\nApprove or reject."
	response, err = agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", response)
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
