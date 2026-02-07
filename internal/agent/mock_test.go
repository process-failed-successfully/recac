package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager. Please generate a JSON list of tickets."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected JSON array response, got: %s", resp)
	}

	if !strings.Contains(resp, "Implement prime number checker") {
		t.Errorf("Expected prime number checker task, got: %s", resp)
	}
}

func TestMockAgent_Send_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Hello, who are you?"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected default mock response, got: %s", resp)
	}
}
