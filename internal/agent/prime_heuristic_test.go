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

	// Verify it detected the scenario and mentions TPM handling tickets
	expectedMsg := "The TPM will handle ticket creation"
	if !strings.Contains(response, expectedMsg) {
		t.Errorf("Expected response to mention %q, got: %s", expectedMsg, response)
	}

	// Verify it DOES NOT import the plan using agent-bridge
	if strings.Contains(response, "agent-bridge import") {
		t.Errorf("Expected response NOT to contain agent-bridge import, got: %s", response)
	}

	// Verify it runs git init
	if !strings.Contains(response, "git init") {
		t.Errorf("Expected response to contain git init, got: %s", response)
	}

	// Verify it creates feature_list.json manually (NEW REQUIREMENT)
	if !strings.Contains(response, "cat <<EOF > feature_list.json") {
		t.Errorf("Expected response to create feature_list.json, got: %s", response)
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

func TestMockAgent_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - Technical Program Manager. Please create the plan."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON content
	expectedJSONSnippet := `"id": "PRIMES"`
	if !strings.Contains(response, expectedJSONSnippet) {
		t.Errorf("Expected response to contain JSON snippet %q, got: %s", expectedJSONSnippet, response)
	}

	// Verify Title format (Must contain ID:[PRIMES] for CLI mapping)
	expectedTitle := `"title": "ID:[PRIMES] Implement prime number script"`
	if !strings.Contains(response, expectedTitle) {
		t.Errorf("Expected response to contain title %q, got: %s", expectedTitle, response)
	}

	expectedDesc := `"description": "Implement a python script 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'."`
	if !strings.Contains(response, expectedDesc) {
		t.Errorf("Expected response to contain description %q, got: %s", expectedDesc, response)
	}
}

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - CODING AGENT. Task: [PRIMES] Implement primes.py"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify dynamic branch usage
	expectedBranch := `BRANCH_NAME="agent/${RECAC_PROJECT_ID:-PRIMES-mock}"`
	if !strings.Contains(response, expectedBranch) {
		t.Errorf("Expected response to define BRANCH_NAME dynamically, got: %s", response)
	}

	// Verify usage of BRANCH_NAME
	if !strings.Contains(response, `git checkout -B "$BRANCH_NAME"`) {
		t.Errorf("Expected response to checkout $BRANCH_NAME, got: %s", response)
	}
	if !strings.Contains(response, `git push --force origin "$BRANCH_NAME"`) {
		t.Errorf("Expected response to push $BRANCH_NAME, got: %s", response)
	}
}
