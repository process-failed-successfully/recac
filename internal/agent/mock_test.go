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

	// Verify that it returns a bash script to prevent NO-OP loops
	if !strings.Contains(response, "```bash") {
		t.Error("Response should contain a bash block")
	}
	if !strings.Contains(response, "echo \"Mock Agent executing default action...\"") {
		t.Error("Response should contain execution echo")
	}
}

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()

	// Prompt that triggers TPM logic (without explicit "tickets" plural)
	prompt := "You are an expert Technical Program Manager (TPM)..."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.Contains(response, "\"id\": \"PROJ-1\"") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", response)
	}
	if strings.Contains(response, "Mock agent response") {
		t.Error("TPM prompt should not return generic mock response")
	}
}

func TestMockAgent_CodingAgent(t *testing.T) {
	agent := NewMockAgent()

	// Test Generic Coding Agent Prompt
	prompt := "## YOUR ROLE - CODING AGENT\n\nTask: Fix bug"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "#!/bin/bash") {
		t.Errorf("Expected bash script response for Coding Agent, got: %s", response)
	}
	if !strings.Contains(response, "Mock Coding Agent") {
		t.Error("Expected mock coding agent echo")
	}

	// Test Primes Scenario
	promptPrimes := "## YOUR ROLE - CODING AGENT\n\nTask: Implement [PRIMES] feature"
	responsePrimes, err := agent.Send(context.Background(), promptPrimes)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(responsePrimes, "cat << 'EOF' > primes.py") {
		t.Error("Expected primes.py creation script")
	}
	if !strings.Contains(responsePrimes, "agent-bridge signal COMPLETED true") {
		t.Error("Expected completion signal")
	}
}

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()

	// Test Initializer Prompt
	prompt := "## YOUR ROLE - INITIALIZER AGENT\n..."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(response, "git init") {
		t.Error("Expected git init in initializer response")
	}
	if !strings.Contains(response, "agent-bridge import") {
		t.Error("Expected agent-bridge import in initializer response")
	}
}

func TestMockAgent_Sanitization(t *testing.T) {
	agent := NewMockAgent()

	// Prompt with characters that could break markdown in fallback response
	prompt := "Check this `code` and \"quotes\"\nNew Line"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Response should contain the code block
	if !strings.Contains(response, "```bash") {
		t.Error("Response should contain bash block")
	}

	// Check content
	// The preview in the echo should not contain backticks or quotes
	// The sanitized prompt should be: "Check this code and quotes New Line"
	expectedPreview := "Check this code and quotes New Line"
	if !strings.Contains(response, expectedPreview) {
		t.Errorf("Response should contain sanitized preview %q, got: %s", expectedPreview, response)
	}

	// Ensure no backticks inside the echo statement (which is inside the code block)
	// We extract the block content
	start := strings.Index(response, "```bash")
	block := response[start:]
	// There should be exactly 2 sets of triple backticks (start and end)
	// But `strings.Count` counts occurrences.
	// The sanitized string is inside the block.
	// Just checking that we don't have stray backticks in the middle.

	// The only backticks should be the code block delimiters.
	// Note: We might have an echo "..." command.

	// Let's just ensure the echo command itself doesn't contain backticks
	// echo "Mock agent response: ... Prompt preview: Check this code..."
	if strings.Contains(block, "Prompt preview: Check this `code`") {
		t.Error("Echo command contains unsanitized backticks")
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
