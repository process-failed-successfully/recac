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

func TestMockAgent_PrimesLoop(t *testing.T) {
	agent := NewMockAgent()

	// Simulate the prompt received after a successful (but empty) commit
	prompt := `
output:
On branch agent/MFLP-2901
nothing to commit, working tree clean
Nothing to commit

primes.py content...
`
	// We need to make sure isPrimesTask triggers. It looks for "primes.py" or "primes.json" or "req-primes".
	prompt += " primes.py "

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the response to contain the completion command
	expectedCmd := "agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true"
	if !strings.Contains(response, expectedCmd) {
		t.Errorf("Expected response to contain completion command '%s', got: %s", expectedCmd, response)
	}
}

func TestMockAgent_ImportCommand(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please initialize the project with agent-bridge import"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expectedCmd := "cat feature_list.json | agent-bridge import"
	if !strings.Contains(response, expectedCmd) {
		t.Errorf("Expected response to contain import command '%s', got: %s", expectedCmd, response)
	}
}
