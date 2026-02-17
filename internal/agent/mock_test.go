package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	// Case 1: Default response
	t.Run("DefaultResponse", func(t *testing.T) {
		prompt := "This is a test prompt"
		response, err := agent.Send(context.Background(), prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "Mock agent response") {
			t.Errorf("Response missing prefix, got: %s", response)
		}
	})

	// Case 2: Prime Python Scenario
	t.Run("PrimePythonScenario", func(t *testing.T) {
		prompt := "Task ID: PROJ-123\nID:[PRIMES] Implement prime calculation logic"
		response, err := agent.Send(context.Background(), prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "primes.py") {
			t.Errorf("Response missing primes.py creation, got: %s", response)
		}
		if !strings.Contains(response, "agent-bridge feature set PROJ-123 --status done --passes true") {
			t.Errorf("Response missing status update, got: %s", response)
		}
	})

	// Case 3: TPM Role
	t.Run("TPMRole", func(t *testing.T) {
		prompt := "You are a Technical Program Manager"
		response, err := agent.Send(context.Background(), prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "agent-bridge signal set PROJECT_SIGNED_OFF true") {
			t.Errorf("Response missing sign-off signal, got: %s", response)
		}
	})

	// Case 4: Execution Phase (Pending)
	t.Run("ExecutionPhase", func(t *testing.T) {
		prompt := `Task ID: PROJ-456
		Here is the feature list:
		[{"id": "PROJ-456", "status": "pending"}]`
		response, err := agent.Send(context.Background(), prompt)

		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if !strings.Contains(response, "agent-bridge feature set PROJ-456 --status done --passes true") {
			t.Errorf("Response missing status update, got: %s", response)
		}
	})
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
