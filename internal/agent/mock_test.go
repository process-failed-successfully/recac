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

func TestMockAgent_ScenarioRouting(t *testing.T) {
	agent := NewMockAgent()

	// Case 1: Initializer Agent (Should return import script)
	promptInitializer := "## YOUR ROLE - INITIALIZER AGENT\nPlease create feature_list.json"
	respInit, err := agent.Send(context.Background(), promptInitializer)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(respInit, "agent-bridge import") {
		t.Errorf("Initializer prompt did not return import script. Got:\n%s", respInit)
	}

	// Case 2: Coding Agent (Has feature_list.json in context, but works on primes.py)
	// This simulates the regression where listing files triggered Initializer logic
	promptCoding := `## YOUR ROLE - CODING AGENT
Context:
ls -la
feature_list.json
primes.py

Task: Implement primes.py`
	respCoding, err := agent.Send(context.Background(), promptCoding)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should NOT return import script
	if strings.Contains(respCoding, "agent-bridge import") {
		t.Errorf("Coding agent prompt FALSE POSITIVE: triggered Initializer import script")
	}

	// Should return implementation
	if !strings.Contains(respCoding, "cat <<EOF > primes.py") {
		t.Errorf("Coding agent prompt failed to return implementation. Got:\n%s", respCoding)
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
