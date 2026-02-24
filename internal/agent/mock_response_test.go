package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_PrimesResponse_Generation(t *testing.T) {
	agent := NewMockAgent()

	// 1. Generation Phase Prompt (Matches AppSpec)
	prompt := "### ID:[PRIMES] Prime Number Script"

	response, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)

	// Verify JSON plan IS present
	assert.Contains(t, response, "ID:[PRIMES] Create Prime Number Script")

	// Verify Bash block IS NOT present (To avoid JSON parse error)
	assert.NotContains(t, response, "```bash")
	assert.NotContains(t, response, "cat << 'EOF' > primes.py")

	assert.False(t, strings.HasPrefix(response, "Mock agent response"))
}

func TestMockAgent_PrimesResponse_Execution(t *testing.T) {
	agent := NewMockAgent()

	// 2. Execution Phase Prompt (Matches Ticket Title)
	prompt := "Task: ID:[PRIMES] Create Prime Number Script"

	response, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)

	// Verify Bash block IS present (Crucial for execution)
	assert.Contains(t, response, "```bash")
	assert.Contains(t, response, "cat << 'EOF' > primes.py")
	assert.Contains(t, response, "python3 primes.py")
	assert.Contains(t, response, "git commit")

	assert.False(t, strings.HasPrefix(response, "Mock agent response"))
}
