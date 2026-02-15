package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)..."
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "\"title\": \"ID:[PRIMES] Implement Prime Number Generator\"", "Should return JSON ticket list")
	assert.NotContains(t, resp, "Mock agent response:", "Should not return default text")
}

func TestMockAgent_Send_Dev(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Create a python script named 'primes.py'. It MUST be python."

	// First call
	resp1, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp1, "cat << 'EOF' > primes.py", "Should return bash script")
	assert.Contains(t, resp1, "git commit -m", "Should include git commit")

	// Second call
	resp2, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp2, "Task Completed", "Should return completion message on second call")
}

func TestMockAgent_Send_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Test authentication"
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response:", "Should return default text for unknown prompt")
}
