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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	// truncateString now appends "..." when string is longer than maxLen
	if got := truncateString(s, 5); got != "hello..." {
		t.Errorf("Expected 'hello...', got '%s'", got)
	}
	if got := truncateString(s, 20); got != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", got)
	}
}
