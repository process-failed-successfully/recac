package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("General Prompt Returns Code Block", func(t *testing.T) {
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

		if !strings.Contains(response, "```bash") {
			t.Errorf("Response missing code block, got: %s", response)
		}
	})

	t.Run("TPM Prompt Returns JSON", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager (TPM)"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.HasPrefix(strings.TrimSpace(response), "[") {
			t.Errorf("Expected JSON array, got: %s", response)
		}
		if strings.Contains(response, "```bash") {
			t.Errorf("TPM response should not contain bash block, got: %s", response)
		}
	})
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
