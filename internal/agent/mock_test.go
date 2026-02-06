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

	// 1. Test "nothing to commit" trigger
	prompt1 := `
Tasks:
[PRIMES] Create Prime Number Script

History:
Agent: I will implement the primes.py script...
System: command output
On branch agent/MFLP-6554
nothing to commit, working tree clean
`
	response1, err := agent.Send(context.Background(), prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response1, "agent-bridge feature set") {
		t.Errorf("Prompt1: Expected completion script, got:\n%s", response1)
	}

	// 2. Test "No changes to commit" trigger (our new echo)
	prompt2 := `
Tasks:
[PRIMES] Create Prime Number Script

History:
System: Command Output:
No changes to commit
`
	response2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response2, "agent-bridge feature set") {
		t.Errorf("Prompt2: Expected completion script, got:\n%s", response2)
	}
}
