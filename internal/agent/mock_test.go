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

func TestMockAgent_Scenarios(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Planner Scenario
	plannerPrompt := "Create a JSON object containing a feature list"
	resp, err := agent.Send(ctx, plannerPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, `"features": [`) {
		t.Errorf("Expected features JSON, got: %s", resp)
	}

	// 2. TPM Scenario (CLI) - even with "primes.py" in text
	tpmPrompt := "You are an expert Technical Program Manager (TPM). Here is the spec: implement primes.py"
	resp, err = agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, `"title": "ID:[PRIMES] Prime Number Implementation"`) {
		t.Errorf("Expected TPM JSON array, got: %s", resp)
	}
	if !strings.HasPrefix(strings.TrimSpace(resp), "[") {
		t.Errorf("Expected response to start with '[', got: %s", resp)
	}

	// 3. Implementation Scenario
	implPrompt := "Please implement the script primes.py now."
	resp, err = agent.Send(ctx, implPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected implementation bash script, got: %s", resp)
	}
	if !strings.Contains(resp, "git config user.email") {
		t.Errorf("Expected git config setup, got: %s", resp)
	}
	if !strings.Contains(resp, "|| echo \"Nothing to commit\"") {
		t.Errorf("Expected idempotent commit, got: %s", resp)
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
