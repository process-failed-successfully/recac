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

	// Updated expectation: Initializer mentions TPM will handle tickets
	expectedMsg := "The TPM will handle ticket creation"
	if !strings.Contains(resp, expectedMsg) {
		t.Errorf("Expected response to contain %q, got: %s", expectedMsg, resp)
	}
}
