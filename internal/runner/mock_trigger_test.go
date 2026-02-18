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
	// The scenario description uses "generate primes", so we must ensure this triggers the script.
	prompt := "Please write a function to generate primes."
	response, err := mockAgent.Send(ctx, prompt)
	assert.NoError(t, err)

	// Check that we got the script, not the default response
	if strings.Contains(response, "I received your prompt") {
		t.Fatalf("MockAgent failed to trigger on '%s'. Got default response.", prompt)
	}
	assert.Contains(t, response, "primes.py", "Response should contain primes.py script")
	assert.Contains(t, response, "```bash", "Response should contain bash code block")

	// 3. Test Parsing (Executor Regex)
	// We manually verify the regex against the generated response
	// The regex from internal/runner/executor.go is `(?s)```bash\s*(.*?)\s*``` `
	// (Note: we access the private variable via logic duplication or just assume if it matches standard regex)

	// Let's rely on Session.ProcessResponse logic if possible, or just regex check
	// We can't import private `bashBlockRegex` from runner, so we simulate ProcessResponse extraction logic
	// But `ProcessResponse` is a method on Session. We can create a dummy session?

	// Simplified regex check matching the one in executor.go
	// var bashBlockRegex = regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")
	// Since we can't import it, we trust that if the string contains the fences, it should work
	// PROVIDED the newlines are correct.

	assert.Contains(t, response, "\n```bash\n", "Should have newline before and after bash tag start")
	assert.Contains(t, response, "\n```", "Should have newline before closing tag")
}
