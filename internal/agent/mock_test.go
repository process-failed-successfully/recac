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
	prompt := "Please implement the [PRIMES] task."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "I will implement the prime number script") {
		t.Errorf("Expected prime implementation response, got: %s", response)
	}
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Expected primes.py in response")
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are the Technical Program Manager. Create a plan."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Technical Program Manager") && !strings.Contains(response, "analyzed the requirements") {
		// My mock implementation returns "I have analyzed the requirements..."
		t.Errorf("Expected TPM response, got: %s", response)
	}
	if !strings.Contains(response, "primes.json") {
		t.Errorf("Expected json plan")
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
