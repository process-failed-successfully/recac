package agent

import (
	"context"
	"recac/internal/agent/prompts"
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

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. Construct the prompt using the template and variables similar to the real run
	vars := map[string]string{
		"task_id":          "req-primes-py-exists",
		"task_description": "primes.py exists",
		"exclusive_paths":  "none",
		"read_only_paths":  "all",
		"history":          "",
	}

	prompt, err := prompts.GetPrompt(prompts.CodingAgent, vars)
	if err != nil {
		t.Fatalf("Failed to get prompt: %v", err)
	}

	// 2. Send to MockAgent
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 3. Verify Response
	// We expect the script generation response, which contains "cat << 'EOF' > primes.py"
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		// Log the prompt to see what it looks like
		t.Logf("Prompt Content:\n%s", prompt)
		t.Errorf("MockAgent failed to trigger primes logic.\nResponse: %s", response)
	}
}

func TestMockAgent_PrimesScenario_Metadata(t *testing.T) {
	agent := NewMockAgent()

	// Prompt without "primes.py" but with metadata
	prompt := "Do something else.\n\n<!-- RECAC_METADATA: task_id=req-primes-py-exists -->"

	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("MockAgent failed to trigger primes logic via metadata.\nResponse: %s", response)
	}
}
