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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_PrimesCompletion(t *testing.T) {
	agent := NewMockAgent()

	// Prompt simulating the runner loop output where "nothing to commit" occurred
	// It must contain both [PRIMES] (to trigger the scenario) and "nothing to commit" (to trigger the fix)
	prompt := `
Tasks:
[PRIMES] Create Prime Number Script

History:
Agent: I will implement the primes.py script...
System: command output
On branch agent/MFLP-6554
nothing to commit, working tree clean
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the agent to detect completion and try to mark the feature as done
	if strings.Contains(response, "agent-bridge feature set") {
		// Pass
	} else {
		t.Errorf("Expected completion script, got:\n%s", response)
	}
}
