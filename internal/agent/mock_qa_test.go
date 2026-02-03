package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_QA_Manager_Logic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test 1: QA Agent Signal
	qaPrompt := "You are the QA Agent. YOUR ROLE: QA Agent. Please verify the changes."
	resp, err := agent.Send(ctx, qaPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA agent to signal QA_PASSED, but got:\n%s", resp)
	}

	// Test 2: Manager Agent Signal
	managerPrompt := "You are the Manager Agent. YOUR ROLE: Manager Agent. Please review the project."
	resp, err = agent.Send(ctx, managerPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected Manager agent to signal PROJECT_SIGNED_OFF, but got:\n%s", resp)
	}

	// Test 3: Normal Prompt (Fallback)
	normalPrompt := "Just a normal prompt."
	resp, err = agent.Send(ctx, normalPrompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if strings.Contains(resp, "agent-bridge signal") {
		t.Errorf("Expected normal prompt NOT to signal, but got:\n%s", resp)
	}
	if !strings.Contains(resp, "echo \"Acknowledged\"") {
		t.Errorf("Expected normal prompt to return acknowledgment, but got:\n%s", resp)
	}
}
