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

func TestMockAgent_TicketGeneration(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM)... Application Specification: Create a prime number checker."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it's valid JSON
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Errorf("Failed to parse response as JSON: %v. Response: %s", err, response)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty JSON array")
	}

	if result[0]["type"] != "Epic" {
		t.Errorf("Expected Epic, got %v", result[0]["type"])
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
