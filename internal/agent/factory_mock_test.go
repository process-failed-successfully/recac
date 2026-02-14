package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent_MockProvider(t *testing.T) {
	agent, err := NewAgent("mock", "", "mock-model", "/tmp", "test-project")
	require.NoError(t, err)
	assert.NotNil(t, agent)
	_, ok := agent.(*MockAgent)
	assert.True(t, ok, "Agent should be of type *MockAgent")
}
