package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Default Response", func(t *testing.T) {
		prompt := "This is a test prompt that is long enough to be truncated"
		response, err := agent.Send(ctx, prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "Mock agent response") {
			t.Errorf("Response missing prefix, got: %s", response)
		}

		if !strings.Contains(response, "I received your prompt") {
			t.Errorf("Response missing body, got: %s", response)
		}
	})

	t.Run("TPM Heuristic", func(t *testing.T) {
		prompt := "You are a Technical Program Manager. Please create tickets in JSON format."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "[")
		assert.Contains(t, resp, "id")
		assert.Contains(t, resp, "PRIMES")
		assert.Contains(t, resp, "Repo: https://github.com/process-failed-successfully/recac-jira-e2e")
	})

	t.Run("Architect Heuristic", func(t *testing.T) {
		prompt := "You are a Lead Software Architect. Please create tickets in JSON format."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "[")
		assert.Contains(t, resp, "id")
		assert.Contains(t, resp, "PRIMES")
	})

	t.Run("Coding Heuristic", func(t *testing.T) {
		prompt := "Please write a Python script to generate prime numbers."
		resp, err := agent.Send(ctx, prompt)
		assert.NoError(t, err)
		assert.Contains(t, resp, "def is_prime(n):")
		assert.Contains(t, resp, "primes.json")
	})
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
