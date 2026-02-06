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

func TestMockAgent_Roles(t *testing.T) {
	agent := NewMockAgent()

	// 1. Test TPM Role detection
	tpmPrompt := "You are an expert Technical Program Manager (TPM)... Application Specification: ... Implement a python script..."
	response, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("TPM prompt incorrectly triggered Developer response")
	}
	if !strings.Contains(response, `"title": "ID:[PRIMES] Implement Prime Number Script"`) {
		t.Error("TPM prompt failed to return ticket JSON")
	}

	// 2. Test Developer Role detection
	devPrompt := "Implement a python script named 'primes.py'"
	response, err = agent.Send(context.Background(), devPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Developer prompt failed to trigger implementation response")
	}
}

func TestMockAgent_CodingAgent_Disambiguation(t *testing.T) {
	agent := NewMockAgent()

	// Construct a prompt that matches the real Coding Agent prompt
	// It contains "feature_list.json" (triggers Rule 1)
	// It contains "QA" and "Review" (triggers Rule 3)
	// It contains "Implement a python script" (triggers Rule 2)
	// It starts with "## YOUR ROLE - CODING AGENT" (disambiguator)
	prompt := `
## YOUR ROLE - CODING AGENT

STEP 1: GET YOUR BEARINGS
cat feature_list.json | head -50

STEP 2: CHOOSE AND IMPLEMENT
Description: Implement a python script named 'primes.py' that calculates all prime numbers...

COMMUNICATE WITH MANAGER
2. **Quality Assurance**: agent-bridge qa
3. **Manager Review**: agent-bridge manager
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should match Rule 2 (Developer)
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding Agent prompt failed to trigger implementation response. Got: %s", response)
	}

	// Should NOT match Rule 1 (JSON)
	if strings.Contains(response, `"type": "Task"`) {
		t.Error("Coding Agent prompt incorrectly triggered TPM/JSON response")
	}

	// Should NOT match Rule 3 (LGTM)
	if strings.Contains(response, "LGTM") {
		t.Error("Coding Agent prompt incorrectly triggered QA/Review response")
	}
}

func TestMockAgent_CodingAgent_PrimesKeywordOnly(t *testing.T) {
	agent := NewMockAgent()

	// Construct a prompt that simulates a missing full description but has keywords
	// Contains "primes.py"
	// Contains "QA" (from instructions)
	// Does NOT contain "Implement a python script"
	// Has "YOUR ROLE - CODING AGENT"
	prompt := `
## YOUR ROLE - CODING AGENT

STEP 2: CHOOSE AND IMPLEMENT
Description: Write code for primes.py to generate primes.json

COMMUNICATE WITH MANAGER
2. **Quality Assurance**: agent-bridge qa
`

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should match Rule 2 (Developer) because of "primes.py" + "CODING AGENT"
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding Agent prompt (keyword only) failed to trigger implementation response. Got: %s", response)
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
