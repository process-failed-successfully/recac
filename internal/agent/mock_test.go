package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Default Fallback
	prompt := "This is a random prompt that should trigger fallback"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Fallback response missing prefix, got: %s", response)
	}

	// 2. Test TPM / Planning Heuristic
	promptTPM := "You are a Technical Program Manager. Please plan the tasks."
	respTPM, err := agent.Send(context.Background(), promptTPM)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respTPM, "\"tickets\":") {
		t.Errorf("TPM response missing tickets JSON, got: %s", respTPM)
	}

	// 3. Test Primes Coding Task Heuristic
	promptPrimes := "Please implement the task ID:[PRIMES] for calculating primes."
	respPrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respPrimes, "#!/bin/bash") {
		t.Errorf("Primes response missing bash script, got: %s", respPrimes)
	}
	if !strings.Contains(respPrimes, "python3 primes.py") {
		t.Errorf("Primes response missing python execution, got: %s", respPrimes)
	}
	if !strings.Contains(respPrimes, "agent-bridge signal PROJECT_SIGNED_OFF") {
		t.Errorf("Primes response missing sign-off signal, got: %s", respPrimes)
	}

	// 4. Test QA / Manager Review Heuristic
	promptQA := "I am the QA Agent. Reviewing the code."
	respQA, err := agent.Send(context.Background(), promptQA)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respQA, "Code looks good") {
		t.Errorf("QA response incorrect, got: %s", respQA)
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
