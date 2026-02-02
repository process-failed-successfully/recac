package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// QA Prompt
	prompt := `
## YOUR ROLE - QA AGENT

Your job is to verify the project.

### INSTRUCTIONS
...
`
	response, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("expected QA success signal, got: %s", response)
	}
}
