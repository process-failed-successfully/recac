package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test TPM/Planning Heuristic
	tpmPrompt := "You are an expert Technical Program Manager (TPM). Please generate tickets."
	resp, err := agent.Send(ctx, tpmPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, `ID:[PRIMES]`, "Should return JSON with PRIMES ID for TPM prompt")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Should return a JSON array")

	// Test Coding Heuristic
	codingPrompt := "## YOUR ROLE - CODING AGENT\n\nTask: ID:[PRIMES] Implement primes.json"
	resp, err = agent.Send(ctx, codingPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "```bash", "Should return bash block for coding prompt")
	assert.Contains(t, resp, "primes.py", "Should include python script creation")

	// Test Fallback
	genericPrompt := "Hello world"
	resp, err = agent.Send(ctx, genericPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response:", "Should return generic response for unknown prompt")
}

func TestMockAgent_Send_Heuristics_Ambiguous(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test priority: If prompt has both TPM keywords (from history) AND Coding keywords (current role),
	// it should ideally respect the current role.
	// In our simple heuristic, we check specific combinations.

	// Case: History has TPM, but current instruction is CODING AGENT
	// This simulates the full prompt context sent to the agent
	mixedPrompt := `
History:
User: You are a TPM...
Agent: [JSON]...

Current:
## YOUR ROLE - CODING AGENT
Task: ID:[PRIMES] implementation
`
	resp, err := agent.Send(ctx, mixedPrompt)
	assert.NoError(t, err)
	// It should return code because "CODING AGENT" and "PRIMES" are present, triggering the first if-block
	assert.Contains(t, resp, "```bash", "Should prioritize Coding heuristic when CODING AGENT is present")
}
