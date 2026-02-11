package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Default Response", func(t *testing.T) {
		resp, err := agent.Send(ctx, "Hello")
		assert.NoError(t, err)
		assert.Contains(t, resp, "I received your prompt")
	})

	t.Run("TPM Response", func(t *testing.T) {
		resp, err := agent.Send(ctx, "You are an expert Technical Program Manager")
		assert.NoError(t, err)
		assert.Contains(t, resp, "ID:[PRIMES]")
		assert.Contains(t, resp, "title")
		assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Response should start with JSON array")
	})

	t.Run("Coding Response", func(t *testing.T) {
		resp, err := agent.Send(ctx, "Please Implement Primes")
		assert.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py")
		assert.Contains(t, resp, "```bash")
	})

	t.Run("QA Response", func(t *testing.T) {
		resp, err := agent.Send(ctx, "Please QA this code")
		assert.NoError(t, err)
		assert.Contains(t, resp, "LGTM")
	})
}
