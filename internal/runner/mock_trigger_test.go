package runner

import (
	"context"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockAgentTrigger verifies that the MockAgent correctly triggers and returns
// a bash script that the Executor can parse.
func TestMockAgentTriggerAndParse(t *testing.T) {
	// 1. Setup
	mockAgent := agent.NewMockAgent()
	ctx := context.Background()

	// 2. Test Triggering (Broad matching)
	scenarios := []string{
		"Please write a function to generate primes.",
		"Task: Implement generator function.", // Matches ticket summary
	}

	for _, prompt := range scenarios {
		response, err := mockAgent.Send(ctx, prompt)
		assert.NoError(t, err)

		// Check that we got the script, not the default response
		if strings.Contains(response, "I received your prompt") {
			t.Fatalf("MockAgent failed to trigger on '%s'. Got default response.", prompt)
		}
		assert.Contains(t, response, "primes.py", "Response should contain primes.py script for prompt: "+prompt)
		assert.Contains(t, response, "```bash", "Response should contain bash code block for prompt: "+prompt)
	}

	// 3. Test Parsing (Executor Regex)
	// We verify parsing on the LAST response obtained in the loop
	// (Since we are testing multiple prompts, we assume the last one is also valid for format checking)

	// Re-fetch response for a known good prompt to verify format
	goodPrompt := "Please write a function to generate primes."
	response, _ := mockAgent.Send(ctx, goodPrompt)

	assert.Contains(t, response, "\n```bash\n", "Should have newline before and after bash tag start")
	assert.Contains(t, response, "\n```", "Should have newline before closing tag")
}
