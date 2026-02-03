package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please run agent-bridge import to initialize."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > feature_list.json") {
		t.Errorf("Expected feature list creation, got: %s", resp)
	}
	if !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import command, got: %s", resp)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - QA AGENT\n\nPlease run tests."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA_PASSED signal, got: %s", resp)
	}
}

func TestMockAgent_Manager(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - PROJECT MANAGER\n\nDecide."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected PROJECT_SIGNED_OFF signal, got: %s", resp)
	}
}

func TestMockAgent_NoOp(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Just a random chat."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "# no-op") {
		t.Errorf("Expected # no-op, got: %s", resp)
	}
}
