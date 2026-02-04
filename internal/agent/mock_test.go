package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_CompletionCheck(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: Prompt contains "nothing to commit"
	prompt := "git commit failed: nothing to commit, working tree clean"

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify that it returns the completion commands (feature set passed)
	if !strings.Contains(resp, "agent-bridge feature set 1 --status done --passes true") {
		t.Errorf("Expected response to contain feature completion command, got: %s", resp)
	}
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected response to contain QA_PASSED signal, got: %s", resp)
	}
}

func TestMockAgent_Send_QARole(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: Prompt contains explicit QA role header
	prompt := "## YOUR ROLE - QA AGENT\n\nVerify the project..."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify that it returns the QA signal
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected response to contain QA_PASSED signal, got: %s", resp)
	}

	// Test case: Prompt contains "nothing to commit" AND "QA Agent" in text
	// This simulates the race condition where coding agent sees "nothing to commit" but history has "QA Agent"
	promptMixed := "nothing to commit\n\nPrevious logs: QA Agent said..."

	respMixed, err := agent.Send(ctx, promptMixed)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// It SHOULD return completion commands (feature set passed), NOT just QA signal
	// primarily because the completion check is now higher priority
	if !strings.Contains(respMixed, "agent-bridge feature set 1 --status done --passes true") {
		t.Errorf("Expected response to contain feature completion command (priority check), got: %s", respMixed)
	}
}
