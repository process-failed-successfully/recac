package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Role returns JSON", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager (TPM)..."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "json")
		assert.Contains(t, resp, "Implement Core Features")
		assert.Contains(t, resp, "ID:[PRIMES]")
	})

	t.Run("Coding Agent returns Python", func(t *testing.T) {
		// Use a prompt that triggers coding agent but NOT the "primes" stop heuristic
		prompt := "You are a Coding Agent. Write a python script."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "def is_prime(n):")
		assert.Contains(t, resp, "agent-bridge feature set")
	})

	t.Run("QA Agent returns QA_PASSED", func(t *testing.T) {
		prompt := "Please Approve or Reject this PR."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Equal(t, "QA_PASSED", resp)
	})

	t.Run("Fallback", func(t *testing.T) {
		prompt := "Hello world"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "Mock agent response")
		assert.Contains(t, resp, "I received your prompt")
	})
}
