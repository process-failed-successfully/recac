package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_Tickets(t *testing.T) {
	agent := NewMockAgent()

	// Test TPM Prompt
	prompt := "You are an expert Technical Program Manager. Please generate a list of tickets in JSON format."
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "PRIMES")
	assert.Contains(t, resp, "\"type\": \"Task\"")

	// Test Architect Prompt
	promptArch := "You are a Lead Software Architect. Output the plan as JSON."
	respArch, err := agent.Send(context.Background(), promptArch)
	assert.NoError(t, err)
	assert.Contains(t, respArch, "PRIMES")
}

func TestMockAgent_Send_Coding(t *testing.T) {
	agent := NewMockAgent()

	// Test Coding Prompt
	prompt := "Write a python script to calculate primes."
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	assert.Contains(t, resp, "python3 primes.py")
}

func TestMockAgent_Send_Default(t *testing.T) {
	agent := NewMockAgent()

	// Test Generic Prompt
	prompt := "Hello there."
	resp, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "Hello there")
}
