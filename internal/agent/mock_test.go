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

	tests := []struct {
		name           string
		prompt         string
		expectContains []string
	}{
		{
			name:   "TPM Prompt",
			prompt: "You are a Technical Program Manager",
			expectContains: []string{
				`"title": "Implement Primes Algorithm"`,
				`"type": "Task"`,
			},
		},
		{
			name:   "Coding Prompt",
			prompt: "Create primes.py",
			expectContains: []string{
				"#!/bin/bash",
				"cat <<EOF > primes.py",
				"git commit -m",
			},
		},
		{
			name:   "Clean Git Status",
			prompt: "working tree clean",
			expectContains: []string{
				"Task completed.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			for _, substr := range tt.expectContains {
				if !strings.Contains(resp, substr) {
					t.Errorf("Response for %q should contain %q, got: %s", tt.name, substr, resp)
				}
			}
		})
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
