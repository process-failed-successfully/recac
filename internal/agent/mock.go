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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for Specific Roles/Tasks (E2E Support)

	// 1. Technical Program Manager (Ticket Generation)
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "Application Specification") || strings.Contains(prompt, "Epics")) {
		// Return a valid JSON response for the Prime Number Script scenario (used in smoke tests)
		return `
		[
			{
				"title": "ID:[PRIMES] Prime Number Script",
				"type": "Task",
				"description": "Implement a Python script to calculate prime numbers up to 10,000.",
				"blocked_by": [],
				"acceptance_criteria": [
					"Script 'primes.py' exists",
					"Calculates primes correctly",
					"Outputs 'primes.json'"
				],
				"children": []
			}
		]
		`, nil
	}

	// 2. Initializer Agent (Start Session)
	if strings.Contains(prompt, "INITIALIZER") || strings.Contains(prompt, "Initializer") {
		// Return a script to "import" features (simulating feature extraction)
		return `
		cat <<EOF > feature_list.json
		[
			{
				"id": "req-primes-py-exists",
				"description": "The script 'primes.py' is implemented",
				"status": "pending",
				"priority": "critical"
			}
		]
		EOF

		agent-bridge import feature_list.json
		`, nil
	}

	// 3. Coding Agent (Implementation)
	// Match on role OR specific feature requirement from the smoke test
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "req-script-primes-py-exists") || strings.Contains(prompt, "req-primes-py-exists") {
		return `
		cat <<EOF > primes.py
		import json

		def is_prime(n):
			if n <= 1:
				return False
			for i in range(2, int(n**0.5) + 1):
				if n % i == 0:
					return False
			return True

		primes = [n for n in range(10000) if is_prime(n)]

		with open('primes.json', 'w') as f:
			json.dump({"primes": primes}, f)
		EOF

		python3 primes.py

		# Signal feature completion
		agent-bridge feature set req-script-primes-py-exists passed || echo "Feature set failed"
		`, nil
	}

	// 4. QA Agent / Manager (Verification)
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "Project Manager") {
		return "QA checks passed. agent-bridge signal QA_PASSED true", nil
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
