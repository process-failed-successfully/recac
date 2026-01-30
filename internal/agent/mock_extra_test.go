package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAgent_TicketGeneration(t *testing.T) {
	ag := NewMockAgent()
	ctx := context.Background()

	// 1. Test Regular Prompt
	resp, err := ag.Send(ctx, "Hello world")
	require.NoError(t, err)
	assert.Contains(t, resp, "Mock agent response")
	assert.Contains(t, resp, "Hello world")

	// 2. Test Ticket Generation Prompt
	ticketPrompt := "You are a Technical Program Manager. Generate Jira tickets for app_spec.txt"
	resp, err = ag.Send(ctx, ticketPrompt)
	require.NoError(t, err)

	// Should be JSON
	assert.True(t, strings.HasPrefix(strings.TrimSpace(resp), "{"), "Response should start with {")
	assert.Contains(t, resp, "\"epics\"")
	assert.Contains(t, resp, "ID:[PRIMES]")
}
