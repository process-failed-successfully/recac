package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "test-project")
	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// Verify it's a MockAgent
	_, ok := agent.(*MockAgent)
	assert.True(t, ok, "Expected agent to be of type *MockAgent")
}

func TestMockAgent_ReturnsJSON_ForTPM(t *testing.T) {
	agent := NewMockAgent()

	// Simulate the prompt sent by jira generate-from-spec
	prompt := "You are an expert Technical Program Manager (TPM)... Please output JSON..."

	resp, err := agent.Send(nil, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, `"title": "ID:[PRIMES] Implement Primes"`)
	assert.Contains(t, resp, `"type": "Story"`)

	// Verify it doesn't return the default mock message
	assert.NotContains(t, resp, "Mock agent response")
}
