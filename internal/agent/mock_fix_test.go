package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics_BashBlocks(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Initializer Heuristic
	promptInitializer := "YOUR ROLE - INITIALIZER AGENT"
	respInitializer, err := agent.Send(ctx, promptInitializer)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respInitializer, "```bash") {
		t.Errorf("Initializer response should contain bash block. Got:\n%s", respInitializer)
	}
	if !strings.Contains(respInitializer, "agent-bridge import") {
		t.Errorf("Initializer response should contain agent-bridge import. Got:\n%s", respInitializer)
	}

	// Test Developer Heuristic
	promptDeveloper := "[PRIMES]"
	respDeveloper, err := agent.Send(ctx, promptDeveloper)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respDeveloper, "```bash") {
		t.Errorf("Developer response should contain bash block. Got:\n%s", respDeveloper)
	}
	if !strings.Contains(respDeveloper, "primes.py") {
		t.Errorf("Developer response should contain primes.py logic. Got:\n%s", respDeveloper)
	}
}
