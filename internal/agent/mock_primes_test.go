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
