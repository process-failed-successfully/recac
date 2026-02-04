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

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Prompt that triggers TPM logic (without explicit "tickets" plural)
	prompt := "You are an expert Technical Program Manager (TPM)..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.Contains(response, "\"id\": \"PROJ-1\"") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", response)
	}
	if strings.Contains(response, "Mock agent response") {
		t.Error("TPM prompt should not return generic mock response")
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
