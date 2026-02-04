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
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_TPM_RepoExtraction(t *testing.T) {
	agent := NewMockAgent()
	// Simulating a prompt that triggers the TPM logic and has text after the Repo URL
	prompt := "Please create tickets for ID:[PRIMES].\nRepo: https://github.com/test/repo\n\n6. **Blockers**: None."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the response to contain the clean URL, but NOT the subsequent text
	expectedURL := "Repo: https://github.com/test/repo\""
	if !strings.Contains(response, expectedURL) {
		t.Errorf("Response should contain clean Repo URL. Got:\n%s", response)
	}

	unexpectedText := "**Blockers**"
	if strings.Contains(response, unexpectedText) {
		t.Errorf("Response should NOT contain trailing prompt text. Got:\n%s", response)
	}
}
