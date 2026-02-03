package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager (TPM)"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Epic: Implement Core Features") {
		t.Errorf("Expected TPM response, got: %s", response)
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement the features" // Generic prompt
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge feature set") {
		t.Errorf("Expected coding response with feature update, got: %s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "YOUR ROLE - QA AGENT"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "QA_PASSED") {
		t.Errorf("Expected QA signal, got: %s", response)
	}
}
