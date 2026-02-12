package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// 1. Default Fallback
	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	// 2. TPM Role
	promptTPM := "## YOUR ROLE - PROJECT MANAGER"
	responseTPM, err := agent.Send(context.Background(), promptTPM)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseTPM, "req-primes-script") {
		t.Errorf("TPM response missing JSON content, got: %s", responseTPM)
	}

	// 3. Initializer Role
	promptInit := "## YOUR ROLE - INITIALIZER AGENT"
	responseInit, err := agent.Send(context.Background(), promptInit)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseInit, "agent-bridge import") {
		t.Errorf("Initializer response missing bash script, got: %s", responseInit)
	}

	// 4. Coding Agent (Primes)
	promptCoding := "Implement a python script named 'primes.py'"
	responseCoding, err := agent.Send(context.Background(), promptCoding)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseCoding, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding response missing primes.py logic, got: %s", responseCoding)
	}

	// 5. QA Agent
	promptQA := "Please REVIEW the code."
	responseQA, err := agent.Send(context.Background(), promptQA)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responseQA, "PROJECT_SIGNED_OFF") {
		t.Errorf("QA response missing sign-off, got: %s", responseQA)
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
