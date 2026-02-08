package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM/Ticket Generation Heuristic
	promptTPM := "You are an expert Technical Program Manager. Please generate tickets for..."
	respTPM, err := agent.Send(ctx, promptTPM)
	assert.NoError(t, err)
	assert.Contains(t, respTPM, `"title":"ID:[PRIMES] Implement Prime Number Service"`, "Response should contain JSON with specific ticket title")
	assert.Contains(t, respTPM, `"type":"Task"`, "Response should contain JSON with Task type")

	// 2. Test Coding Heuristic
	promptCode := "Please implement a python service to check for prime numbers."
	respCode, err := agent.Send(ctx, promptCode)
	assert.NoError(t, err)
	assert.Contains(t, respCode, "cat <<EOF > primes.py", "Response should contain bash command to create primes.py")
	assert.Contains(t, respCode, "git commit -m", "Response should contain git commit command")
	assert.Contains(t, respCode, "I have completed the task", "Response should contain completion signal")

	// 3. Test Default Fallback
	promptChat := "Hello, how are you?"
	respChat, err := agent.Send(ctx, promptChat)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(respChat, "Mock agent response:"), "Response should be the default mock response")
}
