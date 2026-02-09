package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_PrimesImplementation(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test the primes implementation response
	// The mock agent logic checks:
	// if strings.Contains(prompt, "Developer") || strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "[PRIMES]")
	//    and then strings.Contains(prompt, "[PRIMES]")
	response, err := agent.Send(ctx, "Coding Agent [PRIMES]")
	assert.NoError(t, err)

	// Verify that the response contains the correct range
	// We expect "range(1, 10001)" in the python script
	assert.Contains(t, response, "range(1, 10001)", "The response should generate primes up to 10000")
	assert.Contains(t, response, "primes.json", "The response should save to primes.json")

	// Test the initializer response
	// Logic: if strings.Contains(prompt, "Initializer")
	initResponse, err := agent.Send(ctx, "Initializer")
	assert.NoError(t, err)
	assert.Contains(t, initResponse, "up to 10000", "The initializer description should mention 10000")

	// Test the plan response
	// Logic: if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "[PRIMES]")
	planResponse, err := agent.Send(ctx, "Technical Program Manager [PRIMES]")
	assert.NoError(t, err)
	assert.Contains(t, planResponse, "up to 10000", "The plan description should mention 10000")
}
