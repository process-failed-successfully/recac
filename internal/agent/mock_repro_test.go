package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimePython_CollisionScenario(t *testing.T) {
	mock := NewMockAgent()

	// Simulate a prompt that caused the CI failure:
	// 1. It is a Coding Agent prompt (Has "**Feature ID**")
	// 2. It contains "req-primes-py-exists" (Granular feature)
	// 3. It ALSO contains "ID:[PRIMES]" in the history/context (which triggered the old exclusion logic)
	prompt := `
### YOUR ASSIGNED TASK

- **Feature ID**: req-primes-py-exists
- **Description**: primes.py exists

### RECENT HISTORY

[INFO] Ticket Found: ID:[PRIMES] Prime Number Script
[INFO] Starting iteration...
`

	resp, _ := mock.Send(context.Background(), prompt)

	// We expect the implementation script
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent failed to provide implementation in collision scenario.\nGot: %s", resp)
	}
}

func TestMockAgent_PrimePython_TicketGeneration(t *testing.T) {
	mock := NewMockAgent()

	// Simulate Ticket Generation Prompt (AppSpec)
	// Must NOT return the implementation script, but the JSON ticket list
	prompt := `
### ID:[PRIMES] Prime Number Script

CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
`
	resp, _ := mock.Send(context.Background(), prompt)

	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent returned implementation script during ticket generation!")
	}
	if !strings.Contains(resp, `"title": "ID:[PRIMES] Prime Number Script"`) {
		t.Errorf("MockAgent failed to return ticket JSON.\nGot: %s", resp)
	}
}
