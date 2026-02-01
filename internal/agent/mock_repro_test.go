package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer_Reproduction(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Exact string from CI logs (truncated)
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nYou are the FIRST agent in a long-running autonomous development process."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(resp, "no-op to prevent circuit breaker") {
		t.Errorf("MockAgent failed to detect Initializer prompt. Got default response:\n%s", resp)
	}

	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("MockAgent response did not contain feature_list.json creation script. Got:\n%s", resp)
	}
}
