package agent_test

import (
	"context"
	"strings"
	"testing"

	"recac/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestMockAgent_Heuristics(t *testing.T) {
	a := agent.NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM Heuristic
	tpmPrompt := "You are an expert Technical Program Manager (TPM) with deep experience in agile software development. Review the spec."
	resp, err := a.Send(ctx, tpmPrompt)
	assert.NoError(t, err)
	// Must start with [ or { to be valid JSON for the CLI parser
	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "[") || strings.HasPrefix(strings.TrimSpace(resp), "{"), "TPM response must be JSON")
	assert.Contains(t, resp, "ID:[PRIMES]", "Must contain the expected ticket title")

	// 2. Test Coding Agent
	codingPrompt := "YOUR ROLE - CODING AGENT. Implement feature req-script-prints-primes."
	resp, err = a.Send(ctx, codingPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cat <<EOF > primes.py", "Must generate python script")
	assert.Contains(t, resp, "```bash", "Must be wrapped in bash block")

	// 3. Test Loop Breaker
	loopPrompt := "Status: nothing to commit, working tree clean"
	resp, err = a.Send(ctx, loopPrompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "QA_PASSED true", "Must signal QA passed")
	assert.Contains(t, resp, "PROJECT_SIGNED_OFF true", "Must signal project signed off")
}

func TestMockAgent_Heuristics_CaseInsensitive(t *testing.T) {
	a := agent.NewMockAgent()
	ctx := context.Background()

	// Mixed case prompt that SHOULD trigger Coding Agent logic
	prompt := "You are a Coding Agent. Please implement the Prime number generator."
	resp, err := a.Send(ctx, prompt)
	assert.NoError(t, err)
	assert.Contains(t, resp, "cat <<EOF > primes.py", "Must generate python script even with mixed case")
}
