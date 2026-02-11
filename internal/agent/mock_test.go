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
	assert.Equal(t, "Implement basic feature", tickets[0]["title"])
}

func TestMockAgent_Send_Coding(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	prompt := "Implement the requested feature"

	// Iteration 1
	resp1, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp1, "mock_work.txt")
	assert.Contains(t, resp1, "bash")

	// Iteration 2-5 (work)
	for i := 0; i < 4; i++ {
		_, _ = agent.Send(ctx, prompt)
	}

	// Iteration 6 (completion)
	respFinal, err := agent.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, respFinal, "QA_PASSED")
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
