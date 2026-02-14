package agent

import (
	"context"
	"encoding/json"
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
// It returns a mock response that acknowledges the prompt or uses heuristics to simulate specific behaviors
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptLower := strings.ToLower(prompt)

	// Initializer Agent
	// The prompt usually contains "Initializer Agent" or context about setting up the project
	if strings.Contains(promptLower, "initializer agent") {
		// Need to import features. The prompt usually contains the spec or repo URL.
		// We'll simulate importing a feature.
		// Memory says: "The `MockAgent` E2E scenario script ... explicitly imports the requirement feature via `agent-bridge import`"
		// and "The `MockAgent`'s Initializer response uses a bash script with `cat << 'EOF' | agent-bridge import`"

		featureList := map[string]interface{}{
			"features": []map[string]interface{}{
				{
					"id":          "PRIMES-1",
					"title":       "Generate Primes",
					"description": "Generate prime numbers to a file",
					"status":      "open",
					"type":        "requirement",
				},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)

		return fmt.Sprintf(`#!/bin/bash
cat << 'EOF' | agent-bridge import
%s
EOF
`, string(jsonBytes)), nil
	}

	// TPM Agent (Ticket processing)
	// Memory: "The `recac` CLI `generate-from-spec` command maps the JSON key `title` to the ticket summary... `MockAgent` TPM heuristics must output JSON using `"title"` and `"type"`, and can include `ID:[XYZ]` in the title"
	if strings.Contains(promptLower, "tpm agent") || strings.Contains(promptLower, "generate tickets") {
		// Return a single Task
		tickets := []map[string]string{
			{
				"title":       "Implement Primes Script ID:[PRIMES]",
				"type":        "Task",
				"description": "Write a python script to generate primes.",
			},
		}
		jsonBytes, _ := json.Marshal(tickets)
		return string(jsonBytes), nil
	}

	// Coding Agent
	// Memory: "The `MockAgent` coding heuristic (`internal/agent/mock.go`) must match against artifacts defined in acceptance criteria (e.g., primes.json)"
	if strings.Contains(promptLower, "primes") || strings.Contains(promptLower, "python") {
		// Return python script
		return "```python\nimport json\n\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprimes = [p for p in range(2, 100) if is_prime(p)]\n\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\n```", nil
	}

	// Manager Agent (Review/Sign-off)
	if strings.Contains(promptLower, "manager") {
		// Memory: "must explicitly mark features as completed using `agent-bridge feature set ... --status passed` before signaling `PROJECT_SIGNED_OFF`"
		return "agent-bridge feature set PRIMES-1 --status passed --passes=true\nPROJECT_SIGNED_OFF", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
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
