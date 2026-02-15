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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Planning Phase Heuristic", func(t *testing.T) {
		prompt := "You are a Technical Program Manager. create exactly one ticket in JSON format."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "ID:[PRIMES]") {
			t.Errorf("Expected response to contain 'ID:[PRIMES]', got: %s", resp)
		}
		if !strings.Contains(resp, "\"title\":") {
			t.Errorf("Expected response to contain 'title' key, got: %s", resp)
		}
		if !strings.Contains(resp, "```json") {
			t.Errorf("Expected JSON code block, got: %s", resp)
		}
	})

	t.Run("Coding Phase Heuristic", func(t *testing.T) {
		prompt := "Write a python script to calculate prime numbers."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "def is_prime(n):") {
			t.Errorf("Expected python code 'def is_prime(n):', got: %s", resp)
		}
		if !strings.Contains(resp, "```python") {
			t.Errorf("Expected python code block, got: %s", resp)
		}
	})

	t.Run("Fallback", func(t *testing.T) {
		prompt := "Hello world"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected fallback response, got: %s", resp)
		}
	})
}
