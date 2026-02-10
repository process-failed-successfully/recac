package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a mock agent for testing and E2E scenarios
// It returns predefined responses based on heuristics matching the prompt content
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	iterationCount int
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.iterationCount++

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// --- Heuristics ---

	// 1. Initializer / Architect
	// If asked for a plan or features
	if strings.Contains(lowerPrompt, "role - initializer") || strings.Contains(lowerPrompt, "create a plan") {
		return "```json\n{\"features\": [{\"name\": \"Calculate Primes\", \"description\": \"Implement primes.py\"}]}\n```", nil
	}

	// 2. Project Manager
	// If asked to review or sign off
	if strings.Contains(lowerPrompt, "role - project manager") || strings.Contains(lowerPrompt, "review the code") {
		return "APPROVED", nil
	}

	// 3. Technical Program Manager (Ticket Generation)
	// Must be checked BEFORE Coding Agent because the spec (containing 'primes.py') is included in the prompt
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		return "```json\n" + `[
  {
    "title": "Epic: Primes Implementation",
    "description": "Implement prime number generator. Repo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "Story: Implement primes.py",
        "description": "Create a python script to generate primes. Repo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": ["Script runs", "Output is valid JSON"],
        "blocked_by": []
      }
    ]
  }
]` + "\n```", nil
	}

	// 4. Coding Agent (Prime Python Scenario)
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime number script") {
		// Return the python script implementation
		return `Here is the implementation for primes.py:

` + "```bash" + `
cat << 'EOF' > primes.py
import json

primes = []
for num in range(2, 10000):
    for i in range(2, int(num**0.5) + 1):
        if num % i == 0:
            break
    else:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py
` + "```" + `
`, nil
	}

	// 4. QA Agent
	if strings.Contains(lowerPrompt, "role - qa") {
		return "QA_PASSED", nil
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.\n\nPrompt preview: %s...",
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
