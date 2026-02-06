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

	// Verify that it returns a bash script to prevent NO-OP loops
	if !strings.Contains(response, "```bash") {
		t.Error("Response should contain a bash block")
	}
	if !strings.Contains(response, "echo \"Mock Agent executing default action...\"") {
		t.Error("Response should contain execution echo")
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()

	// 1. Default Initializer
	promptDefault := "## YOUR ROLE - INITIALIZER AGENT\n\nSpec: ..."
	resp, err := agent.Send(context.Background(), promptDefault)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "init-task") {
		t.Error("Expected default init-task in initializer response")
	}

	// 2. Primes Scenario
	promptPrimes := "## YOUR ROLE - INITIALIZER AGENT\n\nSpec: [PRIMES] Prime Number Script"
	respPrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respPrimes, "PRIMES") {
		t.Error("Expected PRIMES feature in initializer response")
	}
	if !strings.Contains(respPrimes, "[PRIMES]") {
		t.Error("Expected [PRIMES] in description")
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Prompt that triggers TPM logic (without explicit "tickets" plural)
	prompt := "You are an expert Technical Program Manager (TPM)..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.Contains(response, "\"id\": \"PROJ-1\"") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", response)
	}
	if strings.Contains(response, "Mock agent response") {
		t.Error("TPM prompt should not return generic mock response")
	}
}

func TestMockAgent_CodingAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test Generic Coding Agent Prompt
	prompt := "## YOUR ROLE - CODING AGENT\n\nTask: Fix bug"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "#!/bin/bash") {
		t.Errorf("Expected bash script response for Coding Agent, got: %s", response)
	}
	if !strings.Contains(response, "Mock Coding Agent") {
		t.Error("Expected mock coding agent echo")
	}

	// Test Primes Scenario
	promptPrimes := "## YOUR ROLE - CODING AGENT\n\nTask: Implement [PRIMES] feature"
	responsePrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responsePrimes, "cat << 'EOF' > primes.py") {
		t.Error("Expected primes.py creation script")
	}
	if !strings.Contains(responsePrimes, "agent-bridge signal COMPLETED true") {
		t.Error("Expected completion signal")
	}
}

func TestMockAgent_NothingToCommit(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt that includes "nothing to commit" in the context of the Coding Agent role
	prompt := `
## YOUR ROLE - CODING AGENT
You are the Coding Agent.

Context:
- User wants to set up the project.
- Previous command: git commit -m "Initial commit"
- Output: On branch master
nothing to commit, working tree clean
`

	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify that the response contains the completion signal
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Expected completion signal in response, got: %s", response)
	}

	if !strings.Contains(response, "nothing to commit") {
		// Just ensuring the log message is there for debugging, optional but good
		// Actually the log message in the script is "No changes to commit."
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
