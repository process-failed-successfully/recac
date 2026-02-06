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

	// 1. Detect "Technical Program Manager" Role (Planning Phase)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Application Specification") {
		return `
` + "```json" + `
{
  "project_name": "recac-scenario",
  "features": [
    {
      "id": "PRIMES",
      "type": "Task",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
      "dependencies": []
    }
  ]
}
` + "```" + `
I have analyzed the requirements and created a plan.
`, nil
	}

	// 2. Detect Coding Task for [PRIMES]
	// We check for various triggers to be robust
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "req-implement-primes-py-script") {
		return `
I will implement the prime number script.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run the script
python3 primes.py

# Git operations
# We configure git just in case it wasn't done globally, using local config to avoid errors
git config user.email "mock-agent@recac.com"
git config user.name "Mock Agent"

git add -f primes.json primes.py
git commit -m "Implement primes.py"
` + "```" + `
`, nil
	}

    // 3. Detect Initializer Role (if prompted to set up project)
    // Wrap in markdown to ensure the runner executes it
    if strings.Contains(prompt, "Initialize the project") || strings.Contains(prompt, "INITIALIZER AGENT") {
         return `
` + "```bash" + `
echo "# Project" > README.md
git init
git add README.md
git config user.email "mock-agent@recac.com"
git config user.name "Mock Agent"
git commit -m "Initial commit"
` + "```" + `
`, nil
    }

	// Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

    // Sanitize response to prevent syntax errors if this text ends up in a script (unlikely but safe)
    response = strings.ReplaceAll(response, "`", "'")

    // Return a harmless echo command so the runner doesn't trip "No-Op" circuit breaker
    return response + "\n\n```bash\necho 'Mock agent fallback response'\n```", nil
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
