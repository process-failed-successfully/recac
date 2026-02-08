package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestMockAgent_Commands(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer Command Check
	initPrompt := "Your Role - Initializer Agent"
	resp, err := agent.Send(ctx, initPrompt)
	if err != nil {
		t.Fatalf("Initializer Send failed: %v", err)
	}
	expectedImport := "agent-bridge import < feature_list.json"
	if !strings.Contains(resp, expectedImport) {
		t.Errorf("Initializer response missing corrected import command.\nExpected: %s\nGot: %s", expectedImport, resp)
	}

	// 2. Coding Agent Command Check
	codingPrompt := "Your Role - Coding Agent\nTask: Primes"
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Coding Agent Send failed: %v", err)
	}
	expectedSet := "agent-bridge feature set feature-1 --status done --passes true"
	if !strings.Contains(resp, expectedSet) {
		t.Errorf("Coding Agent response missing corrected feature set command.\nExpected: %s\nGot: %s", expectedSet, resp)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}
