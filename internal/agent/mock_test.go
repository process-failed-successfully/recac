package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics_PrimeScenarioPrompt(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// This is the prompt sent by the E2E manager/CLI for planning
	prompt := `### ID:[PRIMES] Prime Number Script

CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
Do NOT create an Epic. Do NOT create subtasks.
The ID [PRIMES] must map to this single Task.

Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.
...
Repo: https://github.com/test/repo

As a Technical Program Manager, create a JSON plan for this work.
`

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// We expect the JSON plan response, not the Python code response
	// The Python code response starts with "Sure, here is a Python script..."
	// The Planning response starts with "[" (JSON array)

	// If this fails, it means the agent returned the Python code instead of the plan
	if strings.HasPrefix(strings.TrimSpace(resp), "Sure,") {
		t.Fatalf("Agent returned Python code instead of Planning JSON. Response: %s", resp)
	}

	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Response should start with JSON array")
	assert.Contains(t, resp, `"title": "ID:[PRIMES] Implement prime number generator"`)
}
