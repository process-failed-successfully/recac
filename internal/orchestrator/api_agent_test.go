package orchestrator

import (
	"testing"
	"recac/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestGetSetNewAgentFunc(t *testing.T) {
	original := GetNewAgentFunc()
	defer SetNewAgentFunc(original)

	expectedModel := "test-model"
	var receivedModel string

	mockFunc := func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		receivedModel = model
		return nil, nil
	}

	SetNewAgentFunc(mockFunc)

	retFunc := GetNewAgentFunc()
	retFunc("", "", expectedModel, "", "")

	assert.Equal(t, expectedModel, receivedModel)
}
