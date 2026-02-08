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

func TestMockAgent_ProjectManager_SignOff(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - PROJECT MANAGER\n\nTask: Review project..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expected := "agent-bridge signal PROJECT_SIGNED_OFF true --privileged"
	if !strings.Contains(resp, expected) {
		t.Errorf("Expected response to contain '%s', got: %s", expected, resp)
	}
}
