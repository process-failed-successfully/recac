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

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "[EPIC-1]") {
		t.Errorf("Response should contain mock epic JSON, got: %s", response)
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

func TestMockAgent_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a Technical Program Manager... [PRIMES]"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Response should contain PRIMES task, got: %s", response)
	}
}

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	prompt := "YOUR ROLE - CODING AGENT. Create a python script primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expected := []string{
		"cat <<EOF > primes.py",
		"def is_prime(n):",
		"json.dump(primes, f)",
		"python3 primes.py",
		"git add primes.py primes.json",
		"git commit",
		"agent-bridge feature set --id PRIMES",
	}

	for _, exp := range expected {
		if !strings.Contains(response, exp) {
			t.Errorf("Response missing %q, got: %s", exp, response)
		}
	}
}
