package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	t.Run("Default Response", func(t *testing.T) {
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

		if !strings.Contains(response, "echo \"Mock Agent Fallback\"") {
			t.Errorf("Response missing fallback command, got: %s", response)
		}
	})

	t.Run("Initializer Role", func(t *testing.T) {
		prompt := "## YOUR ROLE - INITIALIZER AGENT\n\nTask details..."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge import") {
			t.Errorf("Initializer response missing agent-bridge import, got: %s", response)
		}
		if !strings.Contains(response, "mock-feature-1") {
			t.Errorf("Initializer response missing mock feature, got: %s", response)
		}
	})

	t.Run("Coding Agent Role", func(t *testing.T) {
		prompt := "## YOUR ROLE - CODING AGENT\n\nTask details..."
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge feature set") {
			t.Errorf("Coding Agent response missing feature update, got: %s", response)
		}
	})

	t.Run("Project Manager Role - Pending", func(t *testing.T) {
		prompt := "## YOUR ROLE - PROJECT MANAGER\n\nfeatures are status: \"pending\""
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal COMPLETED false") {
			t.Errorf("PM response should reject pending work, got: %s", response)
		}
	})

	t.Run("Project Manager Role - Done", func(t *testing.T) {
		prompt := "## YOUR ROLE - PROJECT MANAGER\n\nAll features status: \"done\", passes: true"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("PM response should sign off completed work, got: %s", response)
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
