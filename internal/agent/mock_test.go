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

func TestMockAgent_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt that includes both TPM role indicator AND Primes keywords
	// This mimics the prompt sent by 'recac jira generate-from-spec' for the Primes scenario
	prompt := "You are playing the ROLE - TECHNICAL PROGRAM MANAGER. Please analyze the following spec: [PRIMES] Implement Primes"

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// It should return JSON (starting with [), NOT the python script (starting with 'Here is')
	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Response should start with JSON array bracket: %s", resp)
	assert.Contains(t, resp, "req-primes", "Response should contain the primes ticket ID")
	assert.NotContains(t, resp, "def is_prime", "Response should NOT contain python code")
}

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt that includes Primes keywords BUT NOT TPM role
	// This mimics the prompt sent to the Coding Agent
	prompt := "Please implement the following task: [PRIMES] Implement Primes"

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// It should return the python script
	assert.Contains(t, resp, "def is_prime", "Response should contain python code")
	assert.Contains(t, resp, "cat << 'EOF' > primes.py", "Response should contain bash command")
}
