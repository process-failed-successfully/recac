package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockAgent_Scenarios(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	t.Run("Ticket Generation Scenario", func(t *testing.T) {
		prompt := "### ID:[PRIMES] Prime Number Script\nCRITICAL INSTRUCTION: You MUST create exactly ONE ticket."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Verify it's valid JSON
		var tickets []interface{}
		if err := json.Unmarshal([]byte(resp), &tickets); err != nil {
			t.Errorf("Expected valid JSON response, got error: %v\nResponse: %s", err, resp)
		}
		if len(tickets) == 0 {
			t.Error("Expected at least one ticket in JSON")
		}
	})

	t.Run("Execution Scenario", func(t *testing.T) {
		prompt := "### ID:[PRIMES] Prime Number Script\nImplement the script..."
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Verify it contains bash block
		if !strings.Contains(resp, "```bash") {
			t.Error("Expected bash code block in response")
		}
		if !strings.Contains(resp, "primes.py") {
			t.Error("Expected primes.py content in response")
		}
	})

	t.Run("Default Scenario", func(t *testing.T) {
		prompt := "Hello, world!"
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(resp, "Mock agent response") {
			t.Errorf("Expected default mock response, got: %s", resp)
		}
	})
}
