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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_TicketGeneration_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := `
You are a Technical Program Manager.
Generate a ticket plan for the following spec:
### ID:[PRIMES] Prime Number Script
Implement a python script named 'primes.py'
`
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "[PRIMES] Create Prime Number Script") {
		t.Errorf("Expected Primes ticket plan, got: %s", response)
	}
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Expected primes.py in description, got: %s", response)
	}
}
