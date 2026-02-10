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

func TestMockAgent_Primes_DefaultPrompt(t *testing.T) {
	agent := NewMockAgent()

	// Prompt simulating the default "Coding Agent" prompt without a specific task ID
	prompt := "[PRIMES] ... Task ID: Multiple/Not Assigned ... Description: Continue implementing..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return the bash script, not the generic response
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes.py script, got: %s", response)
	}
}

func TestMockAgent_Primes_FeatureID(t *testing.T) {
	agent := NewMockAgent()

	// Prompt simulating prompt with Feature ID but NO primes.py in description
	prompt := "YOUR ROLE - CODING AGENT ... Feature ID: req-primes-py-exists ... Description: Create script."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return the bash script because of Feature ID
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes.py script via Feature ID heuristic, got: %s", response)
	}
}

func TestMockAgent_Primes_JiraTicket(t *testing.T) {
	agent := NewMockAgent()

	// Prompt simulating prompt with Jira Ticket ID but NO primes.py in description
	prompt := "YOUR ROLE - CODING AGENT ... Feature ID: random-id ... Ticket: MFLP-9899 ... Description: Create script."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return the bash script because of MFLP- ticket
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes.py script via MFLP heuristic, got: %s", response)
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
