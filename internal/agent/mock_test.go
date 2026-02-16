package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesSuccess(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate output from test run in a Primes context
	// Include "prime" to trigger the heuristic that causes the loop
	prompt := `
User: Implement Prime Number Generator.
Agent: Here is the code...
User: Run the tests.
System: Command output:
..
----------------------------------------------------------------------
Ran 2 tests in 0.000s

OK
`
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect "Task completed" but currently it returns the bash script because of the loop
	if !strings.Contains(strings.ToLower(resp), "task completed") {
		// Verify it's returning the loop content (bash script)
		if strings.Contains(resp, "def generate_primes") {
			t.Logf("Reproduced loop behavior: Agent returned code instead of completion")
		}
		t.Errorf("Expected 'Task completed' message, got: %s", resp)
	}
}
