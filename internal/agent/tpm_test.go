package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_TPMHeuristic(t *testing.T) {
	agent := NewMockAgent()

	// Prompt that triggers TPM heuristic
	prompt := "Application Specification for [PRIMES]"
	resp, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON structure
	var tickets []map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &tickets); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nResponse: %s", err, resp)
	}

	// Verify required fields
	for i, ticket := range tickets {
		// Check 'type' field
		typ, ok := ticket["type"].(string)
		if !ok || (typ != "Task" && typ != "Story") {
			t.Errorf("Ticket %d missing valid 'type' field (expected 'Task' or 'Story'), got '%v'", i, typ)
		}

		// Check 'title' format
		title, ok := ticket["title"].(string)
		if !ok || !strings.Contains(title, "ID:[PRIMES]") {
			t.Errorf("Ticket %d missing ID prefix in title: %s", i, title)
		}
	}
}
