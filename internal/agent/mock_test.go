package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "You are an expert Technical Program Manager (TPM)..."
	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Verify response is valid JSON
	var tickets []map[string]interface{}
	err = json.Unmarshal([]byte(resp), &tickets)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.NotEmpty(t, tickets)
	assert.Equal(t, "Implement basic feature", tickets[0]["summary"])
}

func TestMockAgent_Send_Generic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "Hello world"
	resp, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "I received your prompt")
}
