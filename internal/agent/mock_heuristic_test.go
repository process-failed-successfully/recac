package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM Phase (Primes)
	tpmPrompt := "You are an expert Technical Program Manager (TPM)... based on the spec for a prime number calculator..."
	resp, err := agent.Send(ctx, tpmPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "ID:[PRIMES]")
	assert.Contains(t, resp, "[{") // Looks like JSON

	// 2. Coding Phase (Primes)
	codingPrompt := "## YOUR ROLE - CODING AGENT\n Please implement the prime number script."
	resp, err = agent.Send(ctx, codingPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "primes.py")
	assert.Contains(t, resp, "range(10000)")
	assert.Contains(t, resp, "```bash")

	// 3. Fallback
	unknownPrompt := "Hello world"
	resp, err = agent.Send(ctx, unknownPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "In mock mode, I would process this request")
}
