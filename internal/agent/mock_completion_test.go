package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_PrimesImplementation_Completion(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt where all tasks are complete
	prompt := `
YOUR ROLE - CODING AGENT
...
**Feature ID**: NONE_ALL_COMPLETE
...
ID:[PRIMES]
`

	response, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Verify we get the completion signal
	assert.Contains(t, response, "agent-bridge signal COMPLETED true")
	assert.NotContains(t, response, "cat << 'EOF' > primes.py")
	assert.NotContains(t, response, "python3 primes.py")
}

func TestMockAgent_PrimesImplementation_Normal(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate a prompt for a normal feature
	prompt := `
YOUR ROLE - CODING AGENT
...
**Feature ID**: FEATURE-123
...
ID:[PRIMES]
`

	response, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Verify we get the implementation script
	assert.Contains(t, response, "cat << 'EOF' > primes.py")
	assert.Contains(t, response, "agent-bridge feature set FEATURE-123")
	assert.NotContains(t, response, "agent-bridge signal COMPLETED true")
}
