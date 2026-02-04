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

func TestMockAgent_TicketGeneration(t *testing.T) {
	agent := NewMockAgent()

	// Simulate TPM prompt
	prompt := "You are an expert Technical Program Manager (TPM)..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "ID:[PRIMES]") {
		t.Errorf("Response should contain ticket ID, got: %s", response)
	}

	if !strings.Contains(response, "Repo: https://github.com/process-failed-successfully/recac-jira-e2e") {
		t.Errorf("Response should contain repo URL, got: %s", response)
	}
}

func TestMockAgent_Implementation(t *testing.T) {
	agent := NewMockAgent()

	// Simulate Implementation prompt
	prompt := "Create a python script named primes.py"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should contain bash command to create file, got: %s", response)
	}

	if !strings.Contains(response, "agent-bridge feature set") {
		t.Errorf("Response should contain agent-bridge command, got: %s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()

	prompt := "YOUR ROLE - QA AGENT"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal set QA_PASSED true") {
		t.Errorf("Response should contain QA signal, got: %s", response)
	}
}

func TestMockAgent_Manager(t *testing.T) {
	agent := NewMockAgent()

	prompt := "YOUR ROLE - PROJECT MANAGER"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "agent-bridge signal set PROJECT_SIGNED_OFF true") {
		t.Errorf("Response should contain PM signal, got: %s", response)
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
