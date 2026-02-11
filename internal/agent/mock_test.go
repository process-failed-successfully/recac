package agent

import (
	"context"
	"testing"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Initializer Agent", func(t *testing.T) {
		prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nPlease create feature_list.json."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "I will create the feature list")
		assert.Contains(t, resp, "cat << 'EOF' > feature_list.json")
	})

	t.Run("Primes Task Implementation", func(t *testing.T) {
		prompt := "## YOUR ROLE - CODING AGENT\n\nImplement the [PRIMES] Prime Number Script"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py")
		assert.Contains(t, resp, "git add primes.py primes.json")
		assert.Contains(t, resp, "is_prime")
	})

	t.Run("Primes Task Generation", func(t *testing.T) {
		prompt := "Generate Jira tickets for [PRIMES] Prime Number Script. Return a JSON list."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "[")
		assert.Contains(t, resp, "\"title\": \"ID:[PRIMES] Prime Number Script\"")
		assert.Contains(t, resp, "\"type\": \"Task\"")
		assert.NotContains(t, resp, "cat << 'EOF' > primes.py")
	})

	t.Run("Primes Task Case Insensitive", func(t *testing.T) {
		prompt := "implement [primes] Prime Number Script"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	})

	t.Run("Primes Task Without Tag", func(t *testing.T) {
		prompt := "Implement the Prime Number Script"
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py", "Should detect task by description")
	})

	t.Run("Coding Agent Role Generic", func(t *testing.T) {
		prompt := "## YOUR ROLE - CODING AGENT\n\nUnknown task."
		resp, err := agent.Send(ctx, prompt)
		require.NoError(t, err)
		assert.Contains(t, resp, "cat << 'EOF' > primes.py", "Should default to implementation logic for Coding Agent")
	})

    // Reproduction of CI Failure
    t.Run("Coding Agent with JSON Context", func(t *testing.T) {
        // This prompt mimics the CI environment where feature_list.json is in context
        // and triggers the "Ticket Generation" logic because of the word "json"
        prompt := "## YOUR ROLE - CODING AGENT\n\nImplement the features from feature_list.json. Ticket ID: [PRIMES]"
        resp, err := agent.Send(ctx, prompt)
        require.NoError(t, err)

        // Should be Implementation (Bash), NOT Ticket Generation (JSON)
        if strings.Contains(resp, "\"title\": \"ID:[PRIMES]") {
            t.Fatalf("Regression: Agent returned Ticket Generation JSON instead of Implementation Script! Response: %s", resp)
        }
        assert.Contains(t, resp, "cat << 'EOF' > primes.py")
    })

	t.Run("Manager Approval", func(t *testing.T) {
		prompt := "## YOUR ROLE - PROJECT MANAGER\n\nReview the code."
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
