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

func TestMockAgent_TicketGeneration_Type(t *testing.T) {
	agent := NewMockAgent()
	// Simulate TPM prompt which includes "Output purely JSON"
	prompt := "Application Specification:\n\n### ID:[PRIMES] Prime Number Script\nRepo: https://example.com/repo\n\nOutput purely JSON"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it returns JSON
	if !strings.HasPrefix(strings.TrimSpace(response), "[") {
		t.Errorf("Expected JSON array response, got: %s", response)
	}

	// Verify content
	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Expected ID:[PRIMES] in response, got: %s", response)
	}
	if !strings.Contains(response, "https://example.com/repo") {
		t.Errorf("Expected repo URL to be preserved, got: %s", response)
	}
}

func TestMockAgent_TicketGenerationWithPrimesContent(t *testing.T) {
	agent := NewMockAgent()
	// This prompt simulates what the Coding Agent receives
	prompt := "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it returns bash code
	if !strings.Contains(response, "```bash") {
		t.Errorf("Expected bash block in response, got: %s", response)
	}

	// Verify content
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected file creation command, got: %s", response)
	}
	if !strings.Contains(response, "git commit") {
		t.Errorf("Expected git commit command, got: %s", response)
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
