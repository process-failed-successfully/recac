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
	prompt := "You are an expert Technical Program Manager... ID:[PRIMES] ..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "ID:[PRIMES]") || !strings.Contains(response, "```json") {
		t.Errorf("TPM heuristic failed, got: %s", response)
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an INITIALIZER AGENT... ID:[PRIMES] ..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "> feature_list.json") || !strings.Contains(response, "```bash") {
		t.Errorf("Initializer heuristic failed, got: %s", response)
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a software engineer... ID:[PRIMES] ..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "primes.py") || !strings.Contains(response, "```bash") {
		t.Errorf("Coding Agent heuristic failed, got: %s", response)
	}
	if !strings.Contains(response, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Errorf("Coding Agent missing feature set done, got: %s", response)
	}
}

func TestMockAgent_Completion(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are a software engineer... All features are marked as done/passing. Please run final verification and signal completion."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal COMPLETED true") {
		t.Errorf("Completion heuristic failed, got: %s", response)
	}
}

func TestMockAgent_QA(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - QA AGENT\n\nYour job is to verify the project."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal QA_PASSED true") {
		t.Errorf("QA Agent heuristic failed, got: %s", response)
	}
}

func TestMockAgent_Manager(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - MANAGER AGENT\n\nManager Review"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge signal PROJECT_SIGNED_OFF true --privileged") {
		t.Errorf("Manager Agent heuristic failed, got: %s", response)
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
