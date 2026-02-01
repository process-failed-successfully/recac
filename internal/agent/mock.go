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

	// 0. Ticket Generation Phase (generate-from-spec)
	if strings.Contains(prompt, "generate-from-spec") || strings.Contains(prompt, "Ticket Generation") || strings.Contains(prompt, "ticket plan") {
		return `[
  {
    "id": "MOCK-1",
    "title": "Setup Project Structure",
    "description": "Initialize the project with required files.",
    "type": "Task",
    "status": "Todo",
    "points": 1,
    "priority": "High"
  },
  {
    "id": "MOCK-2",
    "title": "Implement Core Logic",
    "description": "Implement the main functionality.",
    "type": "Story",
    "status": "Todo",
    "points": 3,
    "priority": "Medium",
    "blockers": ["MOCK-1"]
  }
]`, nil
	}

	// 1. Initializer Phase (creation of feature_list.json)
	if strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "Initialize") {
		return `I will initialize the project by creating the feature list.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "Prime Number Script",
  "features": [
    {
      "id": "req-primes-py-exists",
      "description": "Implement prime calculation logic in primes.py",
      "category": "functional",
      "priority": "critical",
      "status": "pending"
    }
  ]
}
EOF

# Check for bridge and import if available
if command -v agent-bridge &> /dev/null; then
    agent-bridge import --file feature_list.json
fi
` + "```" + `
`, nil
	}

	// 2. Implementation Phase (primes.py)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "10,000") || strings.Contains(prompt, "PRIMES") {
		return `I will create the python script to calculate primes as requested.

` + "```bash" + `
# Configure git
git config --global user.email "bot@recac.com"
git config --global user.name "Recac Bot"

# Create the python script
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

# Run the script to generate output
python3 primes.py

# Commit the results
git add primes.py primes.json
git commit -m "Add primes.py and output" || echo "Nothing to commit"

# Update status
if command -v agent-bridge &> /dev/null; then
    agent-bridge update --id req-primes-py-exists --status done
fi
` + "```" + `
`, nil
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
