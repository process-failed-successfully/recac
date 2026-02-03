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

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Check (Creating feature_list.json)
	if strings.Contains(lowerPrompt, "initializer agent") || (strings.Contains(lowerPrompt, "feature_list.json") && strings.Contains(lowerPrompt, "create")) {
		return `Here is the feature list for the project:

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes Project",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Calculate primes less than 10000",
      "status": "pending",
      "steps": [
        "Run primes.py",
        "Verify primes.json exists",
        "Check count is 1229"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `

I have initialized the feature list.
`, nil
	}

	// 2. Coding Agent Check (Implementing Primes)
	// We check for keywords related to the prime-python scenario
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") || strings.Contains(lowerPrompt, "prime number script") {
		return `I will implement the prime number calculation script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the JSON
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "Implement primes calculation" || echo "Nothing to commit"
` + "```" + `

I have implemented the script, generated the output, and committed the changes.
`, nil
	}

	// Default Mock Response
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
