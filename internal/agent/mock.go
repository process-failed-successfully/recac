package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a mock agent for testing and mock mode
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

	// Priority 1: Ticket Generation (e.g. Type: Task)
	if strings.Contains(prompt, "Type: Task") {
		return m.generateTickets(), nil
	}

	// Priority 2: Initialization (agent-bridge import, Feature List)
	// Trigger: "agent-bridge import" OR ("Feature List" AND "initialize" in prompt)
	if strings.Contains(prompt, "agent-bridge import") ||
       (strings.Contains(prompt, "Feature List") && strings.Contains(strings.ToLower(prompt), "initialize")) {
		return m.generateInitialization(), nil
	}

	// Priority 3: Implementation (primes.py, req-primes, [PRIMES])
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "primes.py") ||
       strings.Contains(lowerPrompt, "req-primes") ||
       strings.Contains(prompt, "[PRIMES]") ||
       strings.Contains(prompt, "ID:[PRIMES]") {
		return m.generatePrimesImplementation(), nil
	}

	// Priority 4: Mock Story IDs
	if strings.Contains(prompt, "ID:[MOCK-STORY]") {
		return "Mock story implementation", nil
	}

	// Priority 5: Planning
	if strings.Contains(prompt, "app_spec.txt") {
		return "Planning response", nil
	}

	// Priority 6: QA keywords
	if strings.Contains(prompt, "QA AGENT") {
		return "QA passed", nil
	}

	// Default response (fallback)
	// Includes a no-op command block to prevent No-Op Loop detection if the runner is expecting commands
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\n```",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generateTickets() string {
    return `[
  {
    "id": "ID:[PRIMES]",
    "type": "Epic",
    "title": "Calculate Primes",
    "description": "Implement a Python script to calculate primes up to 10000.",
    "status": "TODO",
    "children": [
        {
            "id": "ID:[MOCK-STORY]",
            "type": "Story",
            "title": "Mock Story",
            "description": "A mock story.",
            "status": "TODO"
        }
    ]
  }
]`
}

func (m *MockAgent) generateInitialization() string {
    return `I will initialize the project with the feature list.

` + "```bash" + `
cat <<EOF > feature_list.json
[
  {
    "name": "Primes Calculation",
    "description": "Calculate primes up to 10000",
    "status": "todo"
  }
]
EOF

# Import the feature list
agent-bridge import feature_list.json
` + "```" + `
`
}

func (m *MockAgent) generatePrimesImplementation() string {
    return `I will implement the primes calculation in Python.

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10001) if is_prime(p)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output
python3 primes.py

# Configure git and commit
git config user.email "bot@recac.local"
git config user.name "Recac Bot"
git add primes.py primes.json
git commit -m "Implement primes calculation" || echo "Nothing to commit"
` + "```" + `
`
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
