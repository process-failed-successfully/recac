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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_Primes_Initializer(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Initializer prompt (contains [PRIMES] from spec and Role header)
	prompt := `## YOUR ROLE - INITIALIZER AGENT
### Application Specification:
### ID:[PRIMES] Prime Number Script
...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return Bash script with agent-bridge import, NOT Implementation
	if !strings.Contains(response, "agent-bridge import") {
		t.Errorf("Expected agent-bridge import in Initializer, got: %s", response)
	}
	if !strings.Contains(response, "cat << 'EOF' > init.sh") {
		t.Errorf("Expected init.sh creation in Initializer, got: %s", response)
	}
	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Received implementation script (primes.py) in Initializer phase!")
	}
}

func TestMockAgent_Primes_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Simulate TPM prompt
	prompt := `## YOUR ROLE - Technical Program Manager
[PRIMES]
Epics
...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.Contains(response, "```json") {
		t.Errorf("Expected JSON response for TPM, got: %s", response)
	}
}

func TestMockAgent_Primes_CodingAgent_FeatureID(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Coding Agent prompt (Contains feature ID but NO [PRIMES] tag)
	prompt := `## YOUR ROLE - CODING AGENT
...
Feature ID: req-must-correctly-identify-prime-
Description: Script calculates primes correctly
...`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return Implementation (Bash with primes.py)
	// Should NOT return ls -la (Generic Fallback)
	if strings.Contains(response, "ls -la") && !strings.Contains(response, "primes.py") {
		t.Errorf("Coding Agent fell back to generic 'ls -la' response instead of implementing primes.py!")
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected primes.py implementation, got: %s", response)
	}
}

func TestMockAgent_Primes_Completion(t *testing.T) {
	agent := NewMockAgent()

	// Simulate prompt with history showing "No changes to commit"
	prompt := `## YOUR ROLE - CODING AGENT
[PRIMES]
...
Output:
No changes to commit
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should detect completion
	if !strings.Contains(response, "Task appears complete") {
		t.Errorf("Expected completion response, got: %s", response)
	}
}
