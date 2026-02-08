package agent

import (
	"context"
	"encoding/json"
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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()
	// Simulate TPM agent prompt for [PRIMES] scenario
	prompt := "ROLE - TECHNICAL PROGRAM MANAGER\nID:[PRIMES]"

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON structure matches what cmd/recac/jira.go expects
	var tickets []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}

	if err := json.Unmarshal([]byte(resp), &tickets); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nResponse: %s", err, resp)
	}

	if len(tickets) != 1 {
		t.Fatalf("Expected 1 ticket, got %d", len(tickets))
	}

	if tickets[0].Title != "Implement Primes Script" {
		t.Errorf("Expected title 'Implement Primes Script', got '%s'", tickets[0].Title)
	}
}
