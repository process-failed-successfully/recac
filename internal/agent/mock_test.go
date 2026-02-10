package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Initializer Agent", func(t *testing.T) {
		prompt := "You are the INITIALIZER AGENT. Please create feature_list.json."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "I will create the feature list")
		assert.Contains(t, resp, "cat << 'EOF' > feature_list.json")
	})

	t.Run("Primes Task", func(t *testing.T) {
		prompt := "Work on task [PRIMES] Prime Number Script"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py")
		assert.Contains(t, resp, "git add primes.py primes.json")
		assert.Contains(t, resp, "is_prime")
	})

	t.Run("Primes Task Case Insensitive", func(t *testing.T) {
		prompt := "Work on task [primes] Prime Number Script"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	})

	t.Run("Manager Approval", func(t *testing.T) {
		prompt := "You are the MANAGER AGENT. Review the code."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "APPROVED")
	})

	t.Run("Default Response", func(t *testing.T) {
		prompt := "Hello world"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "I received your prompt")
		assert.NotContains(t, resp, "cat << 'EOF'")
	})
}
