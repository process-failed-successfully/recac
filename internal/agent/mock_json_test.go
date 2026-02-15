package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_JSON(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// Test Case 1: Architect Prompt (Expect JSON)
	prompt := "You are an expert Technical Program Manager. Please create exactly one ticket in JSON format."
	resp, err := agent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it parses as JSON
	var result []interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Errorf("Expected JSON response, got: %s\nError: %v", resp, err)
	}

	// Test Case 2: Standard Prompt (Expect Text)
	prompt2 := "Hello, who are you?"
	resp2, err := agent.Send(ctx, prompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.HasPrefix(resp2, "Mock agent response:") {
		t.Errorf("Expected standard mock response, got: %s", resp2)
	}
}
