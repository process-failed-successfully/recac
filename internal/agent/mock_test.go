package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Planning", func(t *testing.T) {
		prompt := `You are an expert Technical Program Manager...
Application Specification:
### ID:[PRIMES] Prime Number Script
Repo: https://github.com/test/repo
`
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "ID:[PRIMES]")
		assert.Contains(t, resp, `"type": "Epic"`)
		assert.Contains(t, resp, "https://github.com/test/repo")
		// Should NOT contain python code
		assert.NotContains(t, resp, "def is_prime")
	})

	t.Run("Coding Execution", func(t *testing.T) {
		prompt := `You are a Coding Agent...
Task: Implement primes.py to print prime numbers.
`
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "def is_prime")
		assert.Contains(t, resp, "git commit")
		// Should NOT contain JSON ticket structure
		assert.NotContains(t, resp, `"type": "Epic"`)
	})

	t.Run("Initializer", func(t *testing.T) {
		prompt := `You are the Initializer Agent... initialize the project features.`
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "agent-bridge import")
	})
}
