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

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please implement ID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should contain primes.py creation script")
	}
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Response should contain completion signal")
	}
}

func TestMockAgent_Primes_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager. Please implement ID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "```json") {
		t.Errorf("Response should contain json block")
	}
	if !strings.Contains(response, "Implement Prime Number Script") {
		t.Errorf("Response should contain ticket title")
	}
	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should NOT contain bash script")
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
