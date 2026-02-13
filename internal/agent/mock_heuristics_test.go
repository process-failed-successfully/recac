package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics_Completion(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Prompt simulating idempotency
	prompt := "Running command: git commit -m 'test'\nOutput:\nnothing to commit, working tree clean"

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "agent-bridge signal PROJECT_SIGNED_OFF true")
	assert.Contains(t, resp, "work is already done")
}

func TestMockAgent_Heuristics_PrimePython(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "Please write a prime number script in python"
	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	// Should NOT contain the signal command unless "nothing to commit" is present
	assert.NotContains(t, resp, "agent-bridge signal PROJECT_SIGNED_OFF")
}

func TestMockAgent_Heuristics_Priority(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Prompt has BOTH keywords but also the "nothing to commit" signal
	// We want the completion signal to take precedence
	prompt := "prime python script\nOutput: nothing to commit, working tree clean"

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Expect completion signal
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Error("MockAgent should prioritize completion signal over code generation when work is done")
	}
	assert.Contains(t, resp, "agent-bridge signal PROJECT_SIGNED_OFF")
}
