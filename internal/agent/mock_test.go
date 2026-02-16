package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Send_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test 1: Planning Phase (TPM)
	// Prompt should contain "Technical Program Manager" and "ID:[PRIMES]"
	planningPrompt := "You are an expert Technical Program Manager. Please plan the following: ID:[PRIMES] Create Prime Number Script"
	resp1, err := agent.Send(ctx, planningPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp1, `"type": "Task"`, "Planning prompt should return JSON task list")
	assert.NotContains(t, resp1, "cat << 'EOF'", "Planning prompt should NOT return bash commands")

	// Test 2: Execution Phase (Agent)
	// Prompt should contain "ID:[PRIMES]" but NOT "Technical Program Manager"
	executionPrompt := "You are an AI software engineer. Implement the task: ID:[PRIMES] Create Prime Number Script"
	resp2, err := agent.Send(ctx, executionPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp2, "```bash", "Execution prompt should return bash code block")
	assert.Contains(t, resp2, "cat << 'EOF' > primes.py", "Execution prompt should generate primes.py")
	assert.Contains(t, resp2, "python3 primes.py", "Execution prompt should run the script")

	// Test 3: Execution Phase (Completion)
	// Prompt should contain success message
	completionPrompt := "You are an AI software engineer. Output: Generated 1229 primes. Task: ID:[PRIMES] Create Prime Number Script"
	resp3, err := agent.Send(ctx, completionPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp3, "agent-bridge feature set", "Completion prompt should return agent-bridge command")
	assert.Contains(t, resp3, "done", "Completion prompt should mark feature as done")

	// Test 4: Fallback
	fallbackPrompt := "Hello agent"
	resp4, err := agent.Send(ctx, fallbackPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp4, "Mock agent response", "Fallback prompt should return standard mock response")
}
