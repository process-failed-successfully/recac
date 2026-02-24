package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	assert.Equal(t, "openai/gpt-4o-mini", DefaultAgentModel, "DefaultAgentModel should match expected")
	assert.Equal(t, "openrouter", DefaultAgentProvider, "DefaultAgentProvider should match expected")
}
