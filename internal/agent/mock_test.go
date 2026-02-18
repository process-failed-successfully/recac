package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)... Repo: https://github.com/test/repo"
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "\"title\": \"Implement Prime Number Generator\"")
	assert.Contains(t, resp, "Repo: https://github.com/test/repo")
}

func TestMockAgent_Send_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Write a Python script that..."
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "```bash")
	assert.Contains(t, resp, "Task completed. Tests passed.")
}

func TestMockAgent_Send_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello"
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "I received your prompt")
}
