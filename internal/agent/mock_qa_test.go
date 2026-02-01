package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test QA Prompt Detection
	prompt := "## YOUR ROLE - QA AGENT\n\nYour job is to verify the project."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the response contains the signal command
	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA signal in response, got:\n%s", resp)
	}

	// 2. Test Verification Prompt Detection
	promptVerify := "Please verify the project."
	respVerify, err := agent.Send(ctx, promptVerify)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(respVerify, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA signal in response to verify prompt, got:\n%s", respVerify)
	}

	// 3. Test Normal Prompt (Should fallback)
	promptNormal := "Write a hello world function."
	respNormal, err := agent.Send(ctx, promptNormal)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(respNormal, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Did not expect QA signal in normal response, got:\n%s", respNormal)
	}
}
