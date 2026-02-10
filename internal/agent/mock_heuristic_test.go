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
		prompt := "You are a Technical Program Manager. Create a ticket plan."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, `"type": "Epic"`)
		assert.Contains(t, resp, `"title": "ID:[SETUP] Initial Project Setup"`)
	})

	t.Run("Initializer Agent", func(t *testing.T) {
		prompt := "## YOUR ROLE - INITIALIZER AGENT. Create feature_list.json."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "agent-bridge import")
		assert.Contains(t, resp, "init.sh")
	})

	t.Run("Coding Agent", func(t *testing.T) {
		prompt := "## YOUR ROLE - Coding Agent. Implement feature."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "```bash")
		assert.Contains(t, resp, "Executing mock agent task")
	})

	t.Run("Default Fallback", func(t *testing.T) {
		prompt := "Hello, world!"
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "I received your prompt")
		assert.NotContains(t, resp, "```bash")
	})
}
