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

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_TPM_RepoExtraction(t *testing.T) {
	agent := NewMockAgent()
	// Simulating a prompt that triggers the TPM logic and has text after the Repo URL
	// Updated to include role marker required by stricter heuristic
	prompt := "You are the Technical Program Manager.\nPlease create tickets for ID:[PRIMES].\nRepo: https://github.com/test/repo\n\n6. **Blockers**: None."

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the response to contain the clean URL, but NOT the subsequent text
	expectedURL := "Repo: https://github.com/test/repo\""
	if !strings.Contains(response, expectedURL) {
		t.Errorf("Response should contain clean Repo URL. Got:\n%s", response)
	}

	unexpectedText := "**Blockers**"
	if strings.Contains(response, unexpectedText) {
		t.Errorf("Response should NOT contain trailing prompt text. Got:\n%s", response)
	}
}

func TestMockAgent_Implementation_PrimesJson(t *testing.T) {
	agent := NewMockAgent()
	// Simulating a prompt that asks for "primes.json exists" but doesn't mention "primes.py"
	prompt := "Task Description: primes.json exists and contains correct primes"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// We expect the implementation script (bash), not the generic response
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should contain implementation script. Got generic response:\n%s", response)
	}
}

func TestMockAgent_CodingAgent_Avoids_TPM(t *testing.T) {
	agent := NewMockAgent()
	// Coding Agent prompt contains the ticket title which has the ID
	// But it also contains the role marker
	prompt := "## YOUR ROLE - CODING AGENT\n\nTask: ID:[PRIMES] Prime Number Script\nDescription: primes.py exists"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// It should NOT return the JSON ticket list (TPM response)
	// TPM response contains "type": "Task" or "type": "Epic"
	if strings.Contains(response, "\"type\": \"Task\"") {
		t.Errorf("Coding Agent prompt incorrectly triggered TPM response:\n%s", response)
	}

	// It SHOULD return the implementation (since it has "primes.py")
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Coding Agent prompt failed to trigger implementation response:\n%s", response)
	}
}
