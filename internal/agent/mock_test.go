package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestMockAgent_Repro_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate the prompt that the agent would receive for the Primes task
	prompt := `
You are an AI agent.
Task: ID:[PRIMES] Implement Primes Service
Description: Implement a service that calculates prime numbers.
...
Feature List:
[{"id": "req-primes", "status": "pending", "description": "Implement primes logic"}]
`

	response, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Now we expect the response to be the Execution Phase response (bash script)
	// AND it should contain the primes logic and git config.

	assert.Contains(t, response, "Here is a plan to implement the primes service.")
	assert.Contains(t, response, "#!/bin/bash")

	// Verification of what IS NOW PRESENT
	assert.Contains(t, response, "cat << 'EOF' > primes.py", "Should contain logic to create primes.py")
	assert.Contains(t, response, "def is_prime(n):", "Should contain python code")
	assert.Contains(t, response, "git config", "Should contain git config setup")
	assert.Contains(t, response, "git commit", "Should contain git commit")
}
