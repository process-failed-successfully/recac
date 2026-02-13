package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	agent := NewMockAgent()

	// Construct a prompt that matches both TPM and Initializer heuristics
	// This simulates the actual Initializer prompt which injects the spec containing ID:[PRIMES]
	prompt := `
## YOUR ROLE - INITIALIZER AGENT

You are the FIRST agent...

### Application Specification:

ID:[PRIMES] Create Prime Number Script
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the Initializer response (Bash script with agent-bridge import)
	// NOT the TPM response (JSON array)

	if strings.Contains(response, "agent-bridge import") {
		// Correct behavior
		return
	}

	if strings.TrimSpace(response)[0] == '[' {
		t.Fatalf("MockAgent returned JSON (TPM response) instead of Initializer response. Heuristic priority is wrong.")
	}

	t.Fatalf("MockAgent returned unexpected response: %s", response)
}
