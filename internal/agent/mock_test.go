package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Primes(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Task: [PRIMES] Create a python script named 'primes.py'"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response did not contain bash script for primes.py:\n%s", resp)
	}
	if !strings.Contains(resp, "import json") {
		t.Errorf("Response missing python code:\n%s", resp)
	}
}

func TestMockAgent_Primes_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Simulating a TPM prompt
	prompt := "You are a Technical Program Manager. Task: [PRIMES] Create a python script."
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// It should NOT contain the bash script
	if strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response contained bash script for TPM prompt, expected JSON plan:\n%s", resp)
	}

	// It SHOULD contain JSON list of tickets
	if !strings.Contains(resp, `"id": "PRIMES"`) {
		t.Errorf("Response missing JSON ticket ID 'PRIMES':\n%s", resp)
	}
}

func TestMockAgent_Normal(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Hello"
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Response missing prefix:\n%s", resp)
	}
}
