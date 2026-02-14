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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()

	// Test with trigger
	prompt := "Please solve ID:[PRIMES] Create Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing primes.py creation:\n%s", response)
	}
	if !strings.Contains(response, "python3 primes.py") {
		t.Errorf("Response missing python execution:\n%s", response)
	}
	if !strings.Contains(response, "agent-bridge signal --privileged PROJECT_SIGNED_OFF") {
		t.Errorf("Response missing completion signal:\n%s", response)
	}

	// Test without trigger
	prompt = "Just a regular prompt"
	response, err = agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(response, "primes.py") {
		t.Errorf("Response should not contain primes scenario logic for regular prompt")
	}
}

func TestMockAgent_PlanningScenario(t *testing.T) {
	agent := NewMockAgent()

	// Test with trigger
	prompt := "You are a Technical Program Manager (TPM)."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON structure
	if !strings.HasPrefix(response, "[") || !strings.HasSuffix(response, "]") {
		t.Errorf("Response should be a JSON array, got: %s", response)
	}
	if !strings.Contains(response, "\"id\": \"PRIMES\"") {
		t.Errorf("Response should contain ticket ID PRIMES")
	}
}

func TestMockAgent_Precedence(t *testing.T) {
	agent := NewMockAgent()

	// Prompt containing BOTH TPM and ID:[PRIMES]
	// This simulates the planning phase where the App Spec (with ID) is included in the prompt
	prompt := "You are a Technical Program Manager (TPM). Here is the spec: ID:[PRIMES] Create Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should match Planning Scenario (JSON), NOT Primes Scenario (Bash)
	if !strings.HasPrefix(response, "[") {
		t.Errorf("Expected JSON array for planning prompt, got bash/text starting with: %s", response[:20])
	}
	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response incorrectly contains bash script logic, indicating wrong precedence")
	}
}
