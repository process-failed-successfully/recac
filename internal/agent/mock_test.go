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

func TestMockAgent_SmokeTestLogic(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test Ticket Generation Prompt
	tpmPrompt := "You are an expert Technical Program Manager (TPM)... please create a ticket plan."
	response, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it returns valid JSON
	var tickets []map[string]interface{}
	if err := json.Unmarshal([]byte(response), &tickets); err != nil {
		t.Fatalf("Expected valid JSON ticket plan, got error: %v\nResponse: %s", err, response)
	}

	if len(tickets) == 0 {
		t.Errorf("Expected at least one ticket")
	}

	// Verify ID:[PRIMES] is present
	found := false
	for _, t := range tickets {
		title, _ := t["title"].(string)
		if strings.Contains(title, "ID:[PRIMES]") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected ticket with ID:[PRIMES], got %v", tickets)
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
