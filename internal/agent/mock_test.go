package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()

	// Test default response
	resp, err := agent.Send(context.Background(), "Hello")
	require.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "Hello")

	// Test forced response
	agent.SetResponse("Forced response")
	resp, err = agent.Send(context.Background(), "Hello")
	require.NoError(t, err)
	assert.Equal(t, "Forced response", resp)

	// Clear forced response
	agent.SetResponse("")
}

func TestMockAgent_Heuristics_PrimePython_Coding(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Create a python script named 'primes.py' that calculates prime numbers"
	resp, err := agent.Send(context.Background(), prompt)
	require.NoError(t, err)

	assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	assert.Contains(t, resp, "def is_prime(n):")
	assert.Contains(t, resp, "json.dump")
	assert.Contains(t, resp, "git push")

	// Ensure the script content is present and looks correct
	assert.True(t, strings.Contains(resp, "range(10000)"), "Script should loop to 10000")
}

func TestMockAgent_Heuristics_PrimePython_Planning(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a planning prompt which contains the description but asks for tickets
	prompt := "You are a Technical Program Manager. Break down the following requirement into Jira tickets: Create a python script named 'primes.py' that calculates prime numbers."
	resp, err := agent.Send(context.Background(), prompt)
	require.NoError(t, err)

	// Should return JSON, NOT code
	assert.Contains(t, resp, "[")
	assert.Contains(t, resp, "{")
	assert.Contains(t, resp, "\"summary\"")
	assert.NotContains(t, resp, "cat << 'EOF' > primes.py")
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	assert.Equal(t, "hello", truncateString(s, 5))
	assert.Equal(t, "hello world", truncateString(s, 20))
}
