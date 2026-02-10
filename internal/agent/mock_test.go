package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test Initializer Heuristic
	prompt := "You are the INITIALIZER AGENT."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```bash") {
		t.Errorf("Expected bash block for Initializer, got: %s", response)
	}
	if !strings.Contains(response, "feature_list.json") {
		t.Errorf("Expected feature_list.json creation, got: %s", response)
	}

	// Test Default Fallback
	prompt = "Hello world"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Mock mode") {
		t.Errorf("Expected 'Mock mode' in fallback response, got: %s", response)
	}
}
