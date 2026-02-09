package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Generic(t *testing.T) {
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

func TestMockAgent_E2E_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager (TPM). Please create a Ticket for the work."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "PRIMES") {
		t.Errorf("Expected TPM response to contain PRIMES ticket, got: %s", response)
	}
	if !strings.Contains(response, "\"type\": \"Task\"") {
		t.Errorf("Expected TPM response to contain JSON with Task type, got: %s", response)
	}
	if strings.Contains(response, "\"children\"") {
		t.Errorf("Expected TPM response NOT to contain children, got: %s", response)
	}
}

func TestMockAgent_E2E_Initializer(t *testing.T) {
	agent := NewMockAgent()
	// Initializer prompt contains spec with 'primes.py', which previously triggered Developer mode
	prompt := "YOUR ROLE - INITIALIZER AGENT. Spec: Implement primes.py"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge import") {
		t.Errorf("Expected Initializer response to contain agent-bridge import, got: %s", response)
	}
	if strings.Contains(response, "def get_primes") {
		t.Errorf("Initializer response should NOT contain implementation code")
	}
}

func TestMockAgent_E2E_Developer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "Implement the primes.py script."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected Developer response to contain file creation, got: %s", response)
	}
	if !strings.Contains(response, "def get_primes(n):") {
		t.Errorf("Expected Developer response to contain python code, got: %s", response)
	}
}

func TestMockAgent_E2E_Developer_Done(t *testing.T) {
	agent := NewMockAgent()
	// Simulate prompt where files exist (primes.json present in file list) or commit message present
	prompt := "Implement the primes.py script.\nGit Log:\nImplement primes.py and generate primes.json"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge feature set --status done") {
		t.Errorf("Expected Developer response to signal completion, got: %s", response)
	}
	if strings.Contains(response, "def get_primes(n):") {
		t.Error("Expected Developer response NOT to contain python code when done")
	}
}

func TestMockAgent_E2E_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are the QA AGENT."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("Expected QA response to signal pass, got: %s", response)
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
