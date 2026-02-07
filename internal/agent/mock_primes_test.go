package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate coding agent prompt
	prompt := "YOUR ROLE - CODING AGENT\n\nTask: [PRIMES] Implement Primes Calculation\n\nFile: primes.py"

	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Agent send failed: %v", err)
	}

	// Verify response contains feature updates
	expectedCommands := []string{
		"agent-bridge feature set req-script-prints-primes-up-to-100 --status done --passes true",
		"agent-bridge feature set req-script-is-runnable --status done --passes true",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(response, cmd) {
			t.Errorf("Expected response to contain command: %s\nGot:\n%s", cmd, response)
		}
	}
}

func TestMockAgent_AmbiguousPrompt(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt that includes "PROJECT MANAGER" in the history (e.g., system prompt or previous messages)
	// but is actually targeting the Coding Agent for the [PRIMES] task.
	prompt := `
System: You are the Project Manager.
User: I have defined the task.
System: You are now the CODING AGENT.
Task: [PRIMES] Implement Primes Calculation.
Context: PROJECT MANAGER previously approved the plan.
`
	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Agent send failed: %v", err)
	}

	// Should NOT return PROJECT_SIGNED_OFF
	if strings.Contains(response, "PROJECT_SIGNED_OFF") {
		t.Errorf("Ambiguous prompt incorrectly triggered Manager response (sign-off). Response:\n%s", response)
	}

	// Should return Coding Agent response (primes.py)
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Ambiguous prompt failed to trigger Coding Agent response. Response:\n%s", response)
	}
}
