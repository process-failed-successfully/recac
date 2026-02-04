package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test default echo behavior
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

	// Test TPM behavior
	tpmPrompt := "You are a Technical Program Manager. Please process ID:[TEST-123]..."
	tpmResponse, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send TPM failed: %v", err)
	}
	if !strings.Contains(tpmResponse, "```json") {
		t.Error("TPM response missing JSON code block")
	}
	if !strings.Contains(tpmResponse, "ID:[TEST-123]") {
		t.Errorf("TPM response missing ID, got: %s", tpmResponse)
	}

	// Test QA behavior
	qaPrompt := "You are a QA AGENT..."
	qaResponse, err := agent.Send(context.Background(), qaPrompt)
	if err != nil {
		t.Fatalf("Send QA failed: %v", err)
	}
	if !strings.Contains(qaResponse, "agent-bridge signal QA_PASSED true") {
		t.Errorf("QA response incorrect, got: %s", qaResponse)
	}

	// Test PM behavior
	pmPrompt := "You are a PROJECT MANAGER..."
	pmResponse, err := agent.Send(context.Background(), pmPrompt)
	if err != nil {
		t.Fatalf("Send PM failed: %v", err)
	}
	if !strings.Contains(pmResponse, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("PM response incorrect, got: %s", pmResponse)
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
