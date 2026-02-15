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

	// 1. Test TPM Heuristic
	tpmPrompt := `
CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
The ID [PRIMES] must map to this single Task.
...
CRITICAL INSTRUCTION FOR TICKET GENERATION:
Create a SINGLE Ticket (Task) for this work.
Repo: https://github.com/test/repo
I am a technical program manager. Output json.
`
	resp, err := agent.Send(ctx, tpmPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "ID:[PRIMES]")
	assert.Contains(t, resp, "primes.py")
	assert.Contains(t, resp, "https://github.com/test/repo")
	assert.True(t, strings.Contains(resp, "```json"), "Response should contain json block")

	// 2. Test Coding Heuristic
	codingPrompt := `
ID:[PRIMES] Create Prime Number Script
...
Output a bash block to create primes.py.
`
	resp, err = agent.Send(ctx, codingPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cat << 'EOF' > primes.py")
	assert.Contains(t, resp, "import json")
	assert.Contains(t, resp, "with open('primes.json', 'w') as f:")
	assert.True(t, strings.Contains(resp, "```bash"), "Response should contain bash block")
}
