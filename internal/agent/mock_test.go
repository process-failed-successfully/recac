package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)... Repo: https://github.com/test/repo"

	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "\"type\": \"Epic\"")
	assert.Contains(t, resp, "\"children\": [")
	assert.Contains(t, resp, "https://github.com/test/repo")

	// Basic JSON validation
	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "["), "Response should start with [")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(resp), "]"), "Response should end with ]")
}

func TestMockAgent_Send_Generic(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello world"

	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "Hello world")
}
