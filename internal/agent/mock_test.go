package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// 1. Standard Prompt
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

	// 2. TPM Prompt (Ticket Generation)
	prompt = "You are an expert Technical Program Manager"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "[{\"description\":\"Create a python script") {
		t.Errorf("Expected JSON ticket response, got: %s", response)
	}

	// 3. Initializer Prompt
	prompt = "Please CREATE FEATURE_LIST.JSON"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "prime-numbers") {
		t.Errorf("Expected Feature list JSON, got: %s", response)
	}

	// 4. Loop Breaker Prompt
	prompt = "nothing to commit, working tree clean"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal --privileged QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal, got: %s", response)
	}

	// Reset Agent for Coding Prompt to ensure iteration count starts fresh
	// (Prevents hitting the lowered loop breaker threshold from previous test steps)
	agent = NewMockAgent()

	// 5. Coding Prompt
	prompt = "Implement Primes"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("Expected bash command to create file, got: %s", response)
	}
	if !strings.Contains(response, "git commit -m \"Implement primes\"") {
		t.Errorf("Expected git commit command, got: %s", response)
	}

	// 6. Iteration Loop Breaker
	// Call Send enough times to trigger the loop breaker (iterationCount > 10)
	// Previous calls: 5. Need > 10.
	for i := 0; i < 7; i++ {
		prompt = "Implement Primes Again"
		response, err = agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}
	// Last response should be the signal
	if !strings.Contains(response, "agent-bridge signal --privileged QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal from loop breaker, got: %s", response)
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
