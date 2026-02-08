package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics_JSONStructure(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Initializer Heuristic
	promptInitializer := "YOUR ROLE - INITIALIZER AGENT"
	respInitializer, err := agent.Send(ctx, promptInitializer)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify correct dependencies structure
	if !strings.Contains(respInitializer, `"dependencies": { "depends_on_ids": [] }`) {
		t.Errorf("Initializer response should contain correct dependencies object. Got:\n%s", respInitializer)
	}

	// Verify it does NOT contain the array form
	if strings.Contains(respInitializer, `"dependencies": []`) {
		t.Errorf("Initializer response should NOT contain dependencies array. Got:\n%s", respInitializer)
	}
}
