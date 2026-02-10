package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response based on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Initializer Heuristic
	if contains(prompt, "INITIALIZER AGENT") || contains(prompt, "You are the Initializer") {
		return `#!/bin/bash
# Initialize environment
echo "Initializing..."
touch .env
echo "Done"
`, nil
	}

	// 2. TPM Heuristic (Jira Ticket Generation)
	if contains(prompt, "Technical Program Manager") || contains(prompt, "agile software development") {
		// Return JSON for tickets. Using Title field as required.
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates the first 10,000 prime numbers.",
    "type": "Task",
    "acceptance_criteria": ["Must be efficient", "Output valid JSON"]
  },
  {
    "title": "ID:[TEST-PRIMES] Verify Primes",
    "description": "Verify the output of the generator.",
    "type": "Task"
  }
]`, nil
	}

	// 3. Coding Agent Heuristic
	if contains(prompt, "Coding Agent") || contains(prompt, "Developer") {
		// Primes implementation logic
		if contains(prompt, "PRIMES") || contains(prompt, "prime") {
			// This heuristic tries to output a Python script and also handle the git operations
			// as mentioned in memory "MockAgent must explicitly execute git config..."
			// But for now, we just return the script content if the prompt asks for code.
			// The actual agent loop might expect just the code block or a script.
			return `#!/usr/bin/env python3
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
num = 2
while len(primes) < 10000:
    if is_prime(num):
        primes.append(num)
    num += 1

print(json.dumps({"primes": primes}))
`, nil
		}

		// If prompt asks to commit/push (which the coding agent often does itself via tools)
		// we might need to simulate tool usage or just text.
		// However, standard Coding Agent output is usually code or tool calls.
		// If we are in "smoke test" mode, maybe we return a simple script.
		return "echo 'Coding task complete'", nil
	}

	// 4. QA Agent / Manager Heuristic
	if contains(prompt, "QA AGENT") || contains(prompt, "QA Agent") || contains(prompt, "PROJECT MANAGER") || contains(prompt, "Manager") {
		// Return signal commands as described in memory
		return `agent-bridge signal QA_PASSED true --privileged
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`, nil
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
