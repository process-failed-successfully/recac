package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesPlanningHeuristic(t *testing.T) {
	m := NewMockAgent()

	// Prompt derived from AppSpec in pkg/e2e/scenarios/prime_python.go
	prompt := `### ID:[PRIMES] Prime Number Script

CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
Do NOT create an Epic. Do NOT create subtasks.
The ID [PRIMES] must map to this single Task.

Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.
...
`
	resp, err := m.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "```json") {
		t.Errorf("Response should contain JSON block for planning")
	}
	if !strings.Contains(resp, `"type": "Task"`) {
		t.Errorf("Response should contain Task type")
	}
	if !strings.Contains(resp, `"title":`) {
		t.Errorf("Response should contain title field")
	}
}

func TestMockAgent_PrimesExecutionHeuristic(t *testing.T) {
	m := NewMockAgent()

	prompt := "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000"
	resp, err := m.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Response should contain 'primes.py'")
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should contain bash block to create file")
	}
}
