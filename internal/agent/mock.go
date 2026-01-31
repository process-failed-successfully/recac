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

	// Smart Mock Logic for E2E Scenarios (e.g. prime-python)

	// 1. Ticket Generation (e.g. CLI usage)
	// Checks if prompt asks to create tickets
	if strings.Contains(prompt, "Type: Task") && strings.Contains(prompt, "[PRIMES]") {
		return `Mock agent response:
[
  {
    "id": "PRIMES",
    "type": "Task",
    "summary": "[PRIMES] Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000",
    "parent_id": ""
  }
]`, nil
	}

	// 2. Initialization (Feature List)
	// Checks if prompt asks for initialization or feature list import
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "agent-bridge import") || (strings.Contains(lowerPrompt, "feature list") && strings.Contains(lowerPrompt, "initialize")) {
		return `Mock agent response:
I will initialize the project.
` + "```bash" + `
cat <<EOF > feature_list.json
{
  "project_name": "primes-python",
  "features": [
    {
      "id": "PRIMES",
      "category": "core",
      "priority": "MVP",
      "description": "Calculate primes < 10000",
      "status": "pending",
      "passes": false,
      "steps": ["Create script", "Run script"],
      "dependencies": {
         "depends_on_ids": [],
         "exclusive_write_paths": ["primes.py", "primes.json"],
         "read_only_paths": []
      }
    }
  ]
}
EOF
agent-bridge import feature_list.json
` + "```", nil
	}

	// 3. Implementation (PRIMES)
	// Checks if prompt is about the prime number task
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `Mock agent response:
I will implement the prime number script.
` + "```bash" + `
# Configure git
git config --global user.email "bot@recac.com"
git config --global user.name "Recac Bot"

# Create script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(1, 10001) if is_prime(i)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run script
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement primes"
` + "```", nil
	}

	// 4. Default / Fallback
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\n```",
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
