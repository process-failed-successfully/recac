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

func TestMockAgent_Send_ManagerRole_FalsePositive(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: QA Agent prompt that mentions "QA Report" in instructions
	// This should NOT trigger the Manager role logic
	prompt := "## YOUR ROLE - QA AGENT\n\nGenerate a detailed QA Report..."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it does NOT return PROJECT_SIGNED_OFF
	if strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("QA Agent prompt incorrectly triggered Manager response: %s", resp)
	}

	// Verify it returns QA_PASSED (as it is the QA agent)
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA response, got: %s", resp)
	}
}

func TestMockAgent_Send_ManagerRole_TruePositive(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test case: Actual Manager prompt
	prompt := "## YOUR ROLE - PROJECT MANAGER\n\nApprove or Reject the project based on the QA Report..."

	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it returns PROJECT_SIGNED_OFF
	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected Manager response, got: %s", resp)
	}
}
