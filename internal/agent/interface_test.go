package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "mock-model", "", "test-project")
	assert.NoError(t, err)
	assert.NotNil(t, agent)
	_, ok := agent.(*MockAgent)
	assert.True(t, ok, "expected *MockAgent")
}
