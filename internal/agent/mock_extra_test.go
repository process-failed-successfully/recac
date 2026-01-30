package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_SmartResponse(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test standard response
	resp, err := agent.Send(ctx, "Hello world")
	require.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "Hello world")

	// 2. Test smart response for primes
	prompt := "Implement a python script named 'primes.py' that calculates primes"
	resp, err = agent.Send(ctx, prompt)
	require.NoError(t, err)

	// Check content
	assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	assert.Contains(t, resp, "primes = []")
	assert.Contains(t, resp, "is_prime = [True] * 10000")
	assert.Contains(t, resp, "python3 primes.py")

	// Check that we can extract the python script
	lines := strings.Split(resp, "\n")
	foundPython := false
	for _, line := range lines {
		if strings.Contains(line, "import json") {
			foundPython = true
			break
		}
	}
	assert.True(t, foundPython, "Should contain python code")
}

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "")
	require.NoError(t, err)
	require.NotNil(t, agent)

	_, ok := agent.(*MockAgent)
	assert.True(t, ok, "Should return *MockAgent")
}
