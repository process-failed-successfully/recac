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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test TPM / Planning Heuristic (Should take precedence over execution even if "prime python" is present)
	tpmPrompt := "You are an expert Technical Program Manager. Decompose the spec for Prime Python Script. Repo: https://github.com/foo/bar\n"
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("TPM Send failed: %v", err)
	}
	if !strings.Contains(resp, "ID:[PRIMES]") {
		t.Errorf("TPM response missing ID:[PRIMES], got: %s", resp)
	}
	if !strings.Contains(resp, "https://github.com/foo/bar") {
		t.Errorf("TPM response missing extracted Repo URL, got: %s", resp)
	}
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("TPM response should be JSON array, got: %s", resp)
	}

	// Test Execution Heuristic
	codingPrompt := "Implement the prime number script in python. Repo: https://github.com/foo/bar"
	resp, err = agent.Send(ctx, codingPrompt)
	if err != nil {
		t.Fatalf("Coding Send failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding response missing implementation script, got: %s", resp)
	}
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Coding response should NOT be JSON array, got: %s", resp)
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
