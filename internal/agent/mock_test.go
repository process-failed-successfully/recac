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

func TestMockAgent_PrimesTicket(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Please generate tickets for ID:[PRIMES]..."

	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it's valid JSON
	var tickets []map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &tickets); err != nil {
		t.Fatalf("Failed to parse response as JSON: %v\nResponse: %s", err, resp)
	}

	if len(tickets) == 0 {
		t.Fatal("Expected at least one ticket")
	}

	title, ok := tickets[0]["title"].(string)
	if !ok || !strings.Contains(title, "[PRIMES]") {
		t.Errorf("Expected title to contain [PRIMES], got: %v", tickets[0]["title"])
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
