package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFeatureList_AgentSendError(t *testing.T) {
	ctx := context.Background()
	mockAgent := &MockPlannerAgent{Err: errors.New("agent failure")}

	_, err := GenerateFeatureList(ctx, mockAgent, "Spec content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed to generate plan")
}

func TestGenerateFeatureList_JSONUnmarshalError(t *testing.T) {
	ctx := context.Background()
	mockAgent := &MockPlannerAgent{Response: "{ invalid json"}

	_, err := GenerateFeatureList(ctx, mockAgent, "Spec content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse agent response")
}
