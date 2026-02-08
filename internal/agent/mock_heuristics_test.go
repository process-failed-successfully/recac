package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("TPM Agent", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager ... Application Specification: ... [PRIMES] ..."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "ID:[PRIMES]")
		assert.Contains(t, resp, "primes.py")
		assert.Contains(t, resp, `"type": "Task"`)
	})

	t.Run("Initializer Agent", func(t *testing.T) {
		prompt := "YOUR ROLE - INITIALIZER AGENT ... Create init.sh ..."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "feature_list.json")
		assert.Contains(t, resp, "req-primes-implementation")
		assert.Contains(t, resp, "agent-bridge import")
		assert.Contains(t, resp, "git clone")
	})

	t.Run("Coding Agent Default", func(t *testing.T) {
		prompt := "YOUR ROLE - CODING AGENT ... [PRIMES] ..."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "primes.py")
		assert.Contains(t, resp, "def is_prime(n):")
		assert.Contains(t, resp, "test_primes.py")
		// Default feature ID fallback
		assert.Contains(t, resp, "agent-bridge feature set req-primes-implementation --status done --passes true")
	})

	t.Run("Coding Agent Extracted Feature ID", func(t *testing.T) {
		prompt := "YOUR ROLE - CODING AGENT \n- **Feature ID**: custom-feature-id\n"
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "agent-bridge feature set custom-feature-id --status done --passes true")
	})

	t.Run("QA Agent", func(t *testing.T) {
		prompt := "YOUR ROLE - QA AGENT ..."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "make test")
		assert.Contains(t, resp, "agent-bridge signal QA_PASSED true")
	})

	t.Run("Manager Agent", func(t *testing.T) {
		prompt := "YOUR ROLE - PROJECT MANAGER ..."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "agent-bridge signal PROJECT_SIGNED_OFF true")
	})

	t.Run("Default", func(t *testing.T) {
		prompt := "Hello"
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "Mock agent response")
	})
}
