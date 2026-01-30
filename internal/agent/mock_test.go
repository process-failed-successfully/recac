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

	// Test case: Prompt containing "primes" should trigger the smart response
	prompt := "Please write a python script to calculate primes"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing primes script, got: %s", response)
	}

	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("Response missing execution command, got: %s", response)
	}

	// Test case: Prompt without "primes" should return default mock response
	prompt = "Do something else"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Should not return primes script for unrelated prompt")
	}
	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing default prefix, got: %s", response)
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
