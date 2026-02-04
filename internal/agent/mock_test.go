package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent("test-model", "test-project")

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

func TestMockAgent_TPMResponse(t *testing.T) {
	agent := NewMockAgent("test-model", "test-project")

	// Prompt simulating the E2E test's TPM prompt
	prompt := "You are an expert Technical Program Manager. Please generate a ticket plan."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Define struct locally to match the one expected in cmd/recac/jira.go (without using the actual private struct)
	type ticketNode struct {
		Title              string       `json:"title"`
		Description        string       `json:"description"`
		Type               string       `json:"type"`
		BlockedBy          []string     `json:"blocked_by"`
		AcceptanceCriteria []string     `json:"acceptance_criteria"`
		Children           []ticketNode `json:"children"`
	}

	var tickets []ticketNode
	if err := json.Unmarshal([]byte(response), &tickets); err != nil {
		t.Fatalf("Failed to unmarshal TPM response as JSON: %v\nResponse: %s", err, response)
	}

	if len(tickets) == 0 {
		t.Fatal("Expected at least one ticket in JSON response")
	}

	// Verify ID tag required for E2E verification
	if !strings.Contains(tickets[0].Title, "ID:[PRIMES]") {
		t.Errorf("Expected ticket title to contain ID:[PRIMES], got: %s", tickets[0].Title)
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
