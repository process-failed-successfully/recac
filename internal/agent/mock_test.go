package agent

import (
	"context"
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

func TestMockAgent_PrimePython(t *testing.T) {
	agent := NewMockAgent()

	// 1. Planning Trigger (ID:[PRIMES] + AppSpec)
	planningPrompt := "This contains ID:[PRIMES] and AppSpec..."
	resp, err := agent.Send(context.Background(), planningPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, `"title": "ID:[PRIMES] Create Prime Number Script"`) {
		t.Errorf("Expected Planning JSON, got: %s", resp)
	}

	// 2. Implementation Trigger (Task:[PRIMES] or 'primes.py' + 'create')
	// Case 1: Standard
	implPrompt1 := "Task: [PRIMES] Description: create primes.py"
	resp1, err := agent.Send(context.Background(), implPrompt1)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp1, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Implementation Bash, got: %s", resp1)
	}
	if !strings.Contains(resp1, "git config") {
		t.Errorf("Expected git config in response, got: %s", resp1)
	}

	// Case 2: No Task ID, but has keywords (case insensitive)
	implPrompt2 := "Please cReaTe a python script called Primes.py"
	resp2, err := agent.Send(context.Background(), implPrompt2)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp2, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Implementation Bash (Case Insensitive), got: %s", resp2)
	}
	if !strings.Contains(resp2, "git config") {
		t.Errorf("Expected git config in response (Case Insensitive), got: %s", resp2)
	}
}

func TestMockAgent_Initializer_NotBlockedByPlanner(t *testing.T) {
	agent := NewMockAgent()

	// Simulating the actual Initializer prompt which triggered the bug.
	// It contains "INITIALIZER" (role), "feature_list.json" (task), and the Spec (which contains ID:[PRIMES] and "Specification").
	initializerPrompt := `
## YOUR ROLE - INITIALIZER AGENT

### TASKS:
2. **Create feature_list.json**: Create a complete and detailed list...

### Application Specification:
... ID:[PRIMES] ...
`
	resp, err := agent.Send(context.Background(), initializerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT match Planning trigger (JSON Array)
	if strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("FAIL: Initializer prompt triggered Planning JSON response instead of Bash script.\nResponse: %s", resp)
	}

	// Should match Initializer trigger
	if !strings.Contains(resp, "I will create the feature list") {
		t.Errorf("Expected Initializer response, got: %s", resp)
	}
}

func TestMockAgent_ManagerReview_Approves(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Manager Review prompt
	managerPrompt := `
## YOUR ROLE - PROJECT MANAGER

Review the QA Report...
`
	resp, err := agent.Send(context.Background(), managerPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should contain sign-off signal
	if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
		t.Errorf("Expected Manager sign-off signal, got: %s", resp)
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
