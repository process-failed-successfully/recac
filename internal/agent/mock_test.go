package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Default(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock Agent") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "received: This is a test prompt") {
		t.Errorf("Response missing content echo, got: %s", response)
	}
}

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"TPM", "You are an expert Technical Program Manager (TPM)", "Implement Primes Script"},
		{"Initializer", "## YOUR ROLE - INITIALIZER AGENT", "feature_list.json"},
		{"Coding", "## YOUR ROLE - CODING AGENT", "primes.py"},
		{"QA", "## YOUR ROLE - QA AGENT", "QA_PASSED"},
		{"Manager", "## YOUR ROLE - PROJECT MANAGER", "PROJECT_SIGNED_OFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(ctx, tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			if !strings.Contains(resp, tt.expected) {
				t.Errorf("Response for %s missing expected content '%s', got: %s", tt.name, tt.expected, resp)
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
