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

func TestMockAgent_SmokeTestLogic(t *testing.T) {
	agent := NewMockAgent()

	// Test Ticket Generation Prompt
	prompt1 := "You are an expert Technical Program Manager. Create tickets in Jira. The ID is ID:[PRIMES]. Output JSON."
	resp1, err := agent.Send(context.Background(), prompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "ID:[PRIMES] Create Prime Number Script") {
		t.Errorf("Expected JSON ticket response, got: %s", resp1)
	}

	// Test Implementation Prompt
	prompt2 := "Implement the code for ID:[PRIMES]. Create primes.py."
	resp2, err := agent.Send(context.Background(), prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script response, got: %s", resp2)
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
