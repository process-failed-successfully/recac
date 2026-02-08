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

func TestMockAgent_PrimesCompletion_CorrectID(t *testing.T) {
	agent := NewMockAgent()

	prompt := `
Tasks:
[PRIMES] Create Prime Number Script

History:
System: Command Output:
nothing to commit, working tree clean
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// It should use the specific feature ID, not the generic project ID env var
	expectedID := "req-must-correctly-identify-prime-"
	if !strings.Contains(response, expectedID) {
		t.Errorf("Expected feature ID '%s' in response, got:\n%s", expectedID, response)
	}

	if strings.Contains(response, "$RECAC_PROJECT_ID") {
		t.Errorf("Response should not rely on $RECAC_PROJECT_ID, got:\n%s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()

	prompt := "## YOUR ROLE - QA AGENT\n\nPlease verify the project."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal in response, got:\n%s", response)
	}
}

func TestMockAgent_Manager(t *testing.T) {
	agent := NewMockAgent()

	prompt := "## YOUR ROLE - PROJECT MANAGER\n\nPlease review the project."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal with --privileged in response, got:\n%s", response)
	}
}
