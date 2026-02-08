package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	assert.NoError(t, err)
	assert.Contains(t, response, "Mock agent response")
	assert.Contains(t, response, "I received your prompt")
}

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "### ID:[PRIMES] Please create a python script primes.py"

	response, err := agent.Send(context.Background(), prompt)

	assert.NoError(t, err)

	// Check for key components
	assert.Contains(t, response, "cat << 'EOF' > primes.py")
	assert.Contains(t, response, "def is_prime(n):")
	assert.Contains(t, response, "python3 primes.py")
	assert.Contains(t, response, "git commit")
	assert.Contains(t, response, "agent-bridge feature set PRIMES")
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	assert.Equal(t, "hello", truncateString(s, 5))
	assert.Equal(t, "hello world", truncateString(s, 20))
}
