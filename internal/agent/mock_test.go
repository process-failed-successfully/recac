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

func TestMockAgent_Coding_Primes(t *testing.T) {
	agent := NewMockAgent()
	// Simulate Coding Agent Prompt
	prompt := "## YOUR ROLE - CODING AGENT\n... ### ID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Response missing primes.py generation script")
	}

	if !strings.Contains(response, "agent-bridge feature set PRIMES --status done --passes true") {
		t.Error("Response missing agent-bridge completion signal")
	}
}

func TestMockAgent_TPM_Primes(t *testing.T) {
	agent := NewMockAgent()
	// Simulate TPM Agent Prompt with explicit repo
	expectedRepo := "https://github.com/test-org/test-repo"
	prompt := "You are an expert Technical Program Manager... decompose it into a series... ### ID:[PRIMES] Prime Number Script\nRepo: " + expectedRepo
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "[") || !strings.Contains(response, "{") {
		t.Error("Response does not look like JSON")
	}

	if !strings.Contains(response, `"title": "ID:[PRIMES] Prime Number Script"`) {
		t.Error("Response missing expected JSON title")
	}

	if !strings.Contains(response, expectedRepo) {
		t.Errorf("Response missing expected repo URL: %s", expectedRepo)
	}

	if strings.Contains(response, "cat << 'EOF'") {
		t.Error("TPM response should NOT contain bash script")
	}
}

func TestMockAgent_TPM_Primes_Fallback(t *testing.T) {
	agent := NewMockAgent()
	// Simulate TPM Agent Prompt without repo
	prompt := "You are an expert Technical Program Manager... decompose it into a series... ### ID:[PRIMES] Prime Number Script"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should fallback to default
	expectedRepo := "https://github.com/process-failed-successfully/recac-jira-e2e"
	if !strings.Contains(response, expectedRepo) {
		t.Errorf("Response missing default repo URL: %s", expectedRepo)
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
