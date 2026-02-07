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

func TestMockAgent_RoleConfusion(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a Coding Agent prompt that mentions the Project Manager in context
	prompt := `
## YOUR ROLE - CODING AGENT

The Project Manager has approved the plan.
Task: Implement primes.py.
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the implementation logic (creating primes.py)
	if strings.Contains(response, "PROJECT_SIGNED_OFF") {
		t.Errorf("MockAgent mistakenly triggered Project Manager sign-off for a Coding Agent prompt")
	}

	if !strings.Contains(response, "primes.py") || !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("MockAgent failed to trigger primes.py implementation logic. Got: %s", response)
	}
}

func TestMockAgent_CodingAgentInterception(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a Coding Agent prompt that includes ticket context (which triggers the Ticket heuristic)
	// ensuring lowercase keywords to trigger containsTicketKeywords
	prompt := `
## YOUR ROLE - CODING AGENT

Context:
- ticket: jira-123
- type: story
- Description: Implement primes.py for the epic.

Task: Write the code.
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect implementation logic, NOT JSON ticket list
	if strings.Contains(response, "\"type\": \"Epic\"") {
		t.Errorf("MockAgent mistakenly returned JSON ticket list for Coding Agent prompt")
	}

	if !strings.Contains(response, "cat <<EOF > primes.py") {
		t.Errorf("MockAgent failed to trigger primes.py implementation. Got: %s", response)
	}
}

func TestMockAgent_PrimesTicketGeneration(t *testing.T) {
	agent := NewMockAgent()

	// Prompt simulating the PrimePythonScenario AppSpec
	prompt := `
### ID:[PRIMES] Prime Number Script

CRITICAL INSTRUCTION FOR TICKET GENERATION:
Create a SINGLE Ticket (Task) for this work.
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify we get the single task with ID PRIMES
	if !strings.Contains(response, "\"id\": \"PRIMES\"") {
		t.Errorf("Expected ID: PRIMES in response, got: %s", response)
	}
	if !strings.Contains(response, "\"type\": \"Task\"") {
		t.Errorf("Expected Type: Task in response, got: %s", response)
	}
	if strings.Contains(response, "\"type\": \"Epic\"") {
		t.Errorf("Did not expect Epic in response, got: %s", response)
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
