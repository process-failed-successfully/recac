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

	// === Heuristics for Smoke Test Scenarios ===

	// 1. Technical Program Manager (Tickets)
	// Heuristic: "Technical Program Manager" role AND (tickets OR plan OR spec)
	if strings.Contains(prompt, "Technical Program Manager") &&
		(strings.Contains(prompt, "tickets") || strings.Contains(prompt, "plan") || strings.Contains(prompt, "Specification") || strings.Contains(prompt, "app_spec.txt")) {

		// Return JSON tickets for Primes scenario
		return `[
  {
    "title": "ID:[req-must-correctly-identify-prime-] Implement Prime Number Checker",
    "description": "Create a python script that checks if a number is prime and generates a list of primes up to 10000.",
    "type": "Task",
    "status": "To Do",
    "dependencies": []
  }
]`, nil
	}

	// 2. Initializer Agent (Feature List)
	// Heuristic: "feature_list.json" but NOT "CODING AGENT" (to avoid false positives when Coding Agent reads it)
	if strings.Contains(prompt, "feature_list.json") && !strings.Contains(prompt, "CODING AGENT") {
		return `cat <<EOF > feature_list.json
[
  "req-must-correctly-identify-prime-"
]
EOF
`, nil
	}

	// 3. QA Agent
	// Heuristic: "QA AGENT" or "QA checks"
	if strings.Contains(prompt, "QA AGENT") {
		// Signal QA Passed
		return `QA checks passed.
agent-bridge signal QA_PASSED true --privileged
`, nil
	}

	// 4. Manager Agent (Approval)
	// Heuristic: "Project Manager" but NOT "Developer" (to avoid confusion with Coding Agent context)
	if strings.Contains(prompt, "Project Manager") && !strings.Contains(prompt, "Developer") {
		// Signal Project Signed Off
		return `Approved.
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`, nil
	}

	// 5. Coding Agent (Primes Scenario)
	// Heuristic: "prime" or "python"
	if strings.Contains(strings.ToLower(prompt), "prime") || strings.Contains(strings.ToLower(prompt), "python") {
		return `
# Create primes.py
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Run it to generate the json
python3 primes.py

# Mark feature as done (using positional args for feature ID as required)
agent-bridge feature set req-must-correctly-identify-prime- --status done
`, nil
	}

	// 6. Stop condition for git (prevent infinite loops if git status is clean)
	if strings.Contains(prompt, "No changes to commit") {
		return "Done.", nil
	}

	// Default Response
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
