package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Primes_Completion(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate prompt with success output
	prompt := "You are an AI coding agent. Your task is: Add Unit Tests for Primes. \nOutput from previous command:\n..\n----------------------------------------------------------------------\nRan 2 tests in 0.000s\n\nOK\n"

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// The response should NOT contain a bash block
	assert.NotContains(t, resp, "```bash", "Response should NOT contain a bash block")
	assert.Contains(t, resp, "Task completed", "Response should indicate completion")
}

func TestMockAgent_Primes_Execution(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Simulate initial execution prompt (no success output yet)
	prompt := "You are an AI coding agent. Your task is: Add Unit Tests for Primes."

	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// The response should contain a bash block
	assert.Contains(t, resp, "```bash", "Response should contain a bash block")
	assert.Contains(t, resp, "cat << 'EOF' > primes.py", "Response should include implementation")
}
