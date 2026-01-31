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

	// Heuristic for 'prime-python' scenario (used in CI smoke tests)
	// We check for "Output purely JSON" to distinguish the TPM/Planner prompt from the Coding prompt
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "Output purely JSON") {
		// Extract Repo URL from prompt to maintain consistency
		repoURL := "https://github.com/example/repo"
		if idx := strings.LastIndex(prompt, "Repo: "); idx != -1 {
			repoURL = strings.TrimSpace(prompt[idx+6:])
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: {\"primes\": [2, 3, 5, ...]}\n\nThe script MUST be named 'primes.py'.\nThe output file MUST be named 'primes.json'.\n\nIMPORTANT: You MUST use a bash block to create the file.\nCommit 'primes.json' IMMEDIATELY after creating/running the script. Do NOT leave it untracked.\n\nRepo: %s",
    "type": "Task",
    "children": []
  }
]`, repoURL), nil
	}

	// Heuristic for the 'prime-python' coding task (keywords: "primes.py", "prime numbers")
	if strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "prime numbers") {
		return `I will create the 'primes.py' script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i**0.5) + 1):
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

# Run the script to generate the output file
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
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
