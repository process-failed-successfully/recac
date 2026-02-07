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

	// Heuristics for E2E Tests (Smoke Test)

	// 1. Ticket Generation (TPM Agent)
	if strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Extract Repo URL if possible, otherwise placeholder
		repo := "https://github.com/example/repo"
		if strings.Contains(prompt, "Repo: http") {
			parts := strings.Split(prompt, "Repo: ")
			if len(parts) > 1 {
				// Take the rest of the string after "Repo: "
				afterRepo := parts[1]
				// Split by whitespace to get just the URL, ignoring subsequent text/newlines
				fields := strings.Fields(afterRepo)
				if len(fields) > 0 {
					repo = fields[0]
					// Trim trailing punctuation (dot, comma, etc.) which might appear if the prompt is "Repo: <url>."
					repo = strings.TrimRight(repo, ".,;)")
				}
			}
		}

		return fmt.Sprintf(`
[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\\n\\nRepo: %s",
    "type": "Task",
    "children": [],
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json exists and contains correct primes"
    ]
  }
]
`, repo), nil
	}

	// 2. Implementation (Coding Agent)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "calculate primes") {
		return `
Here is the implementation for the prime number script.

'''bash
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    if n <= 3: return True
    if n % 2 == 0 or n % 3 == 0: return False
    i = 5
    while i * i <= n:
        if n % i == 0 or n % (i + 2) == 0:
            return False
        i += 6
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)

print(f"Calculated {len(primes)} primes.")
EOF

# Run it to generate the json
python3 primes.py

# Git operations
git add primes.py primes.json

# Configure git identity
git config user.email "agent@recac.io"
git config user.name "RECAC Agent"

git commit -m "Add primes.py and generated json"

# Configure auth if API key is present (fallback for missing token in agent shell)
if [ -n "$RECAC_GITHUB_API_KEY" ]; then
    git config --global url."https://${RECAC_GITHUB_API_KEY}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
fi

git push origin HEAD

# Signal completion
agent-bridge signal QA_PASSED true
agent-bridge signal PROJECT_SIGNED_OFF true
'''
`, nil
	}

	// Return a generic mock response for other cases
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
