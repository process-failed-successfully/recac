package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_PrimesResponse(t *testing.T) {
	agent := NewMockAgent()

	// The prompt that triggers the smoke test logic
	prompt := "ID:[PRIMES] Prime Number Script"

	response, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)

	// Verify JSON plan is present
	assert.Contains(t, response, "ID:[PRIMES] Create Prime Number Script")

	// Verify Bash block is present (Crucial for Orchestrator execution)
	assert.Contains(t, response, "```bash")
	assert.Contains(t, response, "cat << 'EOF' > primes.py")
	assert.Contains(t, response, "python3 primes.py")
	assert.Contains(t, response, "git commit")

	// Ensure it's not the default response
	assert.False(t, strings.HasPrefix(response, "Mock agent response"))
}
