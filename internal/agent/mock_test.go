package agent

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "")
	assert.NoError(t, err)
	// Using basic type assertion if *MockAgent isn't exported or similar
	// But based on interface.go reading, it returns NewMockAgent()
	_, ok := agent.(*MockAgent)
	assert.True(t, ok, "Expected agent to be of type *MockAgent")
}
