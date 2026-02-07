package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Initializer_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - INITIALIZER AGENT. Please set up the repo for the prime number script."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it detected the scenario
	if !strings.Contains(response, "create the feature list for the prime number script") {
		t.Errorf("Expected response to mention creating feature list for primes, got: %s", response)
	}

	// Verify it generates the JSON via cat << 'EOF'
	expectedCmd := "cat << 'EOF' | agent-bridge import"
	if !strings.Contains(response, expectedCmd) {
		t.Errorf("Expected response to contain command %q, got: %s", expectedCmd, response)
	}

	// Verify JSON content
	expectedJSONSnippet := `"id": "PRIMES"`
	if !strings.Contains(response, expectedJSONSnippet) {
		t.Errorf("Expected response to contain JSON snippet %q, got: %s", expectedJSONSnippet, response)
	}

	expectedDesc := `"description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'."`
	if !strings.Contains(response, expectedDesc) {
		t.Errorf("Expected response to contain description %q, got: %s", expectedDesc, response)
	}
}

func TestMockAgent_Initializer_Default(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - INITIALIZER AGENT. Please set up the repo."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify it uses the default path
	if strings.Contains(response, "prime number script") {
		t.Errorf("Expected default response, got prime response: %s", response)
	}

	if !strings.Contains(response, "agent-bridge import --file /app/ticket_plan.json") {
		t.Errorf("Expected default response to import from file, got: %s", response)
	}
}
