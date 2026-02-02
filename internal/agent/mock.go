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

	promptLower := strings.ToLower(prompt)

	// Detect "Ticket Generation" prompts (e.g. from generate-from-spec)
	// We check for keywords likely used in the prompt for the TPM agent
	if strings.Contains(promptLower, "technical program manager") ||
		strings.Contains(prompt, "generate-from-spec") {
		return m.generateMockTickets(prompt), nil
	}

	// Detect "Implementation" prompt for PRIMES (Scenario: prime-python)
	if strings.Contains(prompt, "primes.py") || strings.Contains(promptLower, "prime number script") {
		return m.generatePrimesImplementation(), nil
	}

	// Detect "Spec" prompt from unit tests (TestStartCommand)
	// If the prompt is just "Spec" or very short/generic, just complete.
	if strings.Contains(prompt, "Spec") {
		return "Mock Agent: Task Completed.\n```bash\nagent-bridge signal COMPLETED true\n```", nil
	}

	// Detect QA Agent role
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "Mock QA Agent: Verification Passed.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// Detect Project Manager role
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return "Mock Manager: Project Approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Detect Initializer Agent role
	if strings.Contains(strings.ToLower(prompt), "initializer agent") && strings.Contains(prompt, "feature_list.json") {
		return `Mock Initializer: Features Identified.
` + "```bash" + `
cat << 'EOF' > feature_list.json
[
  {
    "id": "mock-feature-1",
    "description": "Mock Feature 1",
    "category": "functional",
    "priority": "high",
    "status": "pending",
    "dependencies": []
  }
]
EOF

cat feature_list.json | agent-bridge import || echo "Import skipped..."
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// generateMockTickets returns a JSON response simulating ticket generation
func (m *MockAgent) generateMockTickets(prompt string) string {
	// If the prompt asks for PRIMES (e.g. ID:[PRIMES]), return the specific task ticket
	if strings.Contains(strings.ToUpper(prompt), "[PRIMES]") || strings.Contains(strings.ToLower(prompt), "prime number script") {
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task"
  }
]`
	}

	// Default Mock Tickets
	return `[
  {
    "title": "ID:[MOCK-EPIC] Implement Mock Feature",
    "description": "Epic for the mock feature implementation.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[MOCK-STORY] Create Basic Structure",
        "description": "Create the basic file structure for the mock feature.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "File structure created",
          "Tests passed"
        ],
        "children": []
      }
    ]
  }
]`
}

func (m *MockAgent) generatePrimesImplementation() string {
	return `I will implement the primes script as requested.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        if all(num % i != 0 for i in range(2, int(num ** 0.5) + 1)):
            primes.append(num)
    return primes

if __name__ == "__main__":
    primes = get_primes(10000)
    with open('primes.json', 'w') as f:
        json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Commit
if [ -d .git ]; then
    git add primes.py primes.json
    git commit -m "Add primes.py and primes.json" || echo \"Nothing to commit\"
fi

# Signal Completion
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
else
    echo "agent-bridge not found, skipping signal"
fi
` + "```"
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
