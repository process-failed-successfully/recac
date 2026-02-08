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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Create a python script primes.py that calculates primes."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "primes.py") {
		t.Errorf("Response missing primes.py code")
	}

	expectedStrings := []string{
		"with open(\"primes.json\", \"w\") as f:",
		"json.dump({\"primes\": primes}, f)",
		"python3 primes.py",
		"git add primes.py primes.json",
		"git commit -m \"Implement primes\"",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(response, expected) {
			t.Errorf("Response missing expected string: %s", expected)
		}
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
