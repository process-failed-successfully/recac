package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Coding_Response(t *testing.T) {
	agent := NewMockAgent()
	// Simulate the prompt for "Implement core logic"
	prompt := "## YOUR ROLE - CODING AGENT\n\nTask: Implement core logic\nTicket: MFLP-9211\n..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// It should return commands, e.g., git commit
	if !strings.Contains(resp, "git commit") {
		t.Errorf("response should contain coding commands, got: %s", resp)
	}
}
