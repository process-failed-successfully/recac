package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristic_Manager_vs_Coding(t *testing.T) {
	agent := NewMockAgent()

	// Simulate a Manager prompt that includes elements triggering the Coding Agent heuristic
	// e.g. "PROJECT MANAGER" header, but also "[PRIMES]" and "primes.py" in the QA report body
	prompt := `## YOUR ROLE - PROJECT MANAGER

### INPUTS
**QA Report:**
Feature ID:[PRIMES] Implement Primes Calculation
File: primes.py
Status: done
Passes: true

### INSTRUCTIONS
...
`

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}

	// We expect the Manager response (Approval), NOT the Coding Agent response (Bash block for primes.py)
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent incorrectly returned Coding Agent response for Manager prompt containing triggers. Response:\n%s", resp)
	}

	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("MockAgent did not return Manager response (PROJECT_SIGNED_OFF). Response:\n%s", resp)
	}
}
