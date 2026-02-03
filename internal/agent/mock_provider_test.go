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
