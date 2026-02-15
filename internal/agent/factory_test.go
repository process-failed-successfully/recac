package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAgent_Mock(t *testing.T) {
	ag, err := NewAgent("mock", "", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, ag)
	assert.IsType(t, &MockAgent{}, ag)
}

func TestNewAgent_Unknown(t *testing.T) {
	ag, err := NewAgent("unknown", "", "", "", "")
	assert.Error(t, err)
	assert.Nil(t, ag)
}
