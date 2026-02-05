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

	// 1. Test TPM Role detection (Should return JSON)
	tpmPrompt := "You are an expert Technical Program Manager (TPM)... Application Specification: ... Implement a python script..."
	response, err := agent.Send(context.Background(), tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if strings.Contains(response, "cat << 'EOF'") {
		t.Error("TPM prompt incorrectly returned bash script instead of JSON")
	}
	if !strings.Contains(response, `"title": "ID:[PRIMES] Implement Prime Number Script"`) {
		t.Error("TPM prompt failed to return correct ticket title in JSON")
	}

	// 2. Test Initializer Role detection (Should return Bash)
	initPrompt := "## YOUR ROLE - INITIALIZER AGENT\n\nCreate feature_list.json"
	response, err = agent.Send(context.Background(), initPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "agent-bridge import") {
		t.Error("Initializer prompt failed to return agent-bridge import command")
	}
	if !strings.Contains(response, "cat << 'EOF' > init.sh") {
		t.Error("Initializer prompt failed to return init.sh creation")
	}

	// 3. Test Developer Role detection
	devPrompt := "Implement a python script named 'primes.py'"
	response, err = agent.Send(context.Background(), devPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Developer prompt failed to trigger implementation response")
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
