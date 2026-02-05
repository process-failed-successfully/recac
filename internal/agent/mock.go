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

	// Heuristic: Check for "TPM" or "Technical Program Manager" role to generate a ticket plan
	if strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Technical Program Manager") {
		// Sub-Heuristic: If the prompt is for the [PRIMES] scenario, return the specific plan
		if strings.Contains(prompt, "[PRIMES]") {
			return `[
  {
    "id": "PRIMES",
    "title": "Create Prime Number Script",
    "description": "Calculate primes < 10000 and output to primes.json. The script MUST be named 'primes.py'.",
    "type": "Task"
  }
]`, nil
		}

		// Default TPM response (Ticket generation)
		return `[
    {
      "title": "Implement Core Feature",
      "description": "Implement the core functionality as requested.",
      "type": "Epic",
      "children": [
        {
          "title": "Setup Project Structure",
          "description": "Initialize the project structure.",
          "type": "Story"
        },
        {
          "title": "Implement Logic",
          "description": "Write the business logic.",
          "type": "Story"
        }
      ]
    }
  ]`, nil
	}

	// Heuristic: Check if this is the Initializer agent
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "feature_list.json") {
		return "Mock Initializer: Creating feature list.\n```bash\necho '[]' > feature_list.json\n```", nil
	}

	// Heuristic: Manager Agent (Check BEFORE QA to avoid false positives from history)
	if strings.Contains(prompt, "Manager") || strings.Contains(prompt, "PROJECT_SIGNED_OFF") {
		return "Mock Manager: Project approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "QA Agent") || strings.Contains(prompt, "QA_PASSED") {
		return "Mock QA: All checks passed.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// Heuristic: Coding Agent for Prime Number Scenario (Smoke Test)
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `Mock Agent: Implementing primes.py
` + "```bash" + `
# Configure Git
git config --global user.email "agent@recac.io"
git config --global user.name "RECAC Agent"

# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run script
python3 primes.py

# Commit and Push
git add primes.py primes.json
git commit -m "Add primes script and output" --author="RECAC Agent <agent@recac.io>"
# Try pushing to current branch, fallback to main if detached
git push origin HEAD || echo "Push skipped"

# Signal Completion
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho 'Mock Agent: Processing request...'\n```",
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
