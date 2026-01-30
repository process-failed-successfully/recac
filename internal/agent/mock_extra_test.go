package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_SmokeTestLogic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Ticket Generation Phase", func(t *testing.T) {
		// Simulate prompt from cmd/recac/jira.go using AppSpec from prime_python.go
		prompt := `...
		### ID:[PRIMES] Prime Number Script
		CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
		...
		Repo: http://example.com`

		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, `"title": "ID:[PRIMES] Create Prime Number Script"`, "Should return JSON ticket plan")
		assert.NotContains(t, resp, "cat << 'EOF'", "Should NOT return python script")
	})

	t.Run("Implementation Phase", func(t *testing.T) {
		// Simulate prompt from orchestrator/agent loop
		prompt := `...
		You are implementing ticket: ID:[PRIMES] Create Prime Number Script
		Description: Create a python script named 'primes.py'. It MUST be python.
		IMPORTANT: You MUST use a bash block to create the file.
		...`

		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py", "Should return python script in bash block")
		assert.Contains(t, resp, "git add primes.py", "Should return git commands")
		// The mock implementation of "JSON Plan" includes "ID:[PRIMES]" in title, so we can't just assert NotContains ID:[PRIMES]
		// But we can assert it starts with text/code, not "["
		assert.False(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Should not return JSON array")
	})

	t.Run("Other Prompts", func(t *testing.T) {
		resp, err := agent.Send(ctx, "Hello world")
		require.NoError(t, err)
		assert.Contains(t, resp, "Mock agent response", "Should return default mock response")
	})
}
