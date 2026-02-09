package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer_Uppercase(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nSome instructions..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "git init") && !strings.Contains(resp, "agent-bridge import") {
		t.Errorf("Expected Initializer response (git init/import), got: %s", resp)
	}
}

func TestMockAgent_ProjectManager_Heuristic(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - PROJECT MANAGER\n\nSome instructions..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected Project Manager response with sign-off command, got: %s", resp)
	}
}

func TestMockAgent_PrimeScenario(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nTask: Implement prime number script..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "Prime Number Generator") {
		t.Errorf("Expected Prime Number Generator in response, got: %s", resp)
	}
}
