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

	// Heuristic for Initializer Agent
	if strings.Contains(promptLower, "initializer agent") {
		return `
I will initialize the project features.

` + "```bash" + `
echo '{"features": ["prime-python"]}' | agent-bridge import
` + "```", nil
	}

	// Heuristic for TPM Agent (Planning)
	if strings.Contains(promptLower, "technical program manager") || strings.Contains(promptLower, "generate ticket") {
		repoURL := "https://github.com/example/repo"
		// Try to extract repo url from prompt if possible, otherwise use default
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			// Take everything after "Repo: "
			afterRepo := parts[1]
			// Split by whitespace or newline to get just the URL
			urlParts := strings.Fields(afterRepo)
			if len(urlParts) > 0 {
				repoURL = urlParts[0]
			}
			// Clean up any trailing punctuation if present (e.g. ` or .)
			repoURL = strings.TrimRight(repoURL, "`.,")
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Create a python script that calculates prime numbers. Repo: %s",
    "type": "Story",
    "acceptance_criteria": [
      "Script named primes.py created",
      "Calculates primes under 10000",
      "Outputs to primes.json"
    ],
    "children": []
  }
]`, repoURL), nil
	}

	// Heuristic for Prime Python Scenario (Execution)
	// We check for "primes.py" but ONLY if it's NOT the planning phase
	if strings.Contains(promptLower, "primes.py") || strings.Contains(promptLower, "prime numbers") {
		return `
I'll create the primes.py script for you.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```", nil
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
