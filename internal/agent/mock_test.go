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
	})

	t.Run("TPM Response", func(t *testing.T) {
		prompt := "You are an expert Technical Program Manager\nID:[PRIMES] Spec"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, `"type": "Epic"`) {
			t.Errorf("Expected JSON response for TPM, got: %s", response)
		}
	})

	t.Run("Coding Agent Response", func(t *testing.T) {
		prompt := "## YOUR ROLE - CODING AGENT\n**Feature ID**: req-test-feature\nDescription: Implement test"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge feature set req-test-feature") {
			t.Errorf("Expected bash script to set feature status, got: %s", response)
		}
		if !strings.Contains(response, "```bash") {
			t.Errorf("Expected bash code block, got: %s", response)
		}
	})

	t.Run("QA Agent Response", func(t *testing.T) {
		prompt := "## YOUR ROLE - QA AGENT\nVerify checks"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected bash script to set QA_PASSED, got: %s", response)
		}
	})

	t.Run("Manager Agent Response", func(t *testing.T) {
		prompt := "## YOUR ROLE - PROJECT MANAGER\nReview report"
		response, err := agent.Send(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected bash script to set PROJECT_SIGNED_OFF, got: %s", response)
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
