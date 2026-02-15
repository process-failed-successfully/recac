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

	// --- 1. Git Commit Message Heuristic ---
	// If the prompt asks for a commit message (often containing "commit message" or "git commit"),
	// return a valid conventional commit string.
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
	}

	// --- 2. TPM / Ticket Generation Heuristic ---
	// If the prompt is for a Technical Program Manager or Architect generating a JSON plan
	if (strings.Contains(lowerPrompt, "technical program manager") ||
		strings.Contains(lowerPrompt, "architect") ||
		strings.Contains(lowerPrompt, "tpm")) &&
		strings.Contains(lowerPrompt, "json") {

		// Return a JSON array of tickets for the prime-python scenario
		return `[
  {
    "id": "TASK-1",
    "type": "Task",
    "summary": "Implement Python Primes Generator",
    "description": "Create a Python script that generates prime numbers up to a specified limit. The script should be efficient and well-documented.",
    "repo_url": "https://github.com/process-failed-successfully/recac-jira-e2e",
    "dependencies": []
  }
]`, nil
	}

	// --- 3. Coding Agent (Prime Python) Heuristic ---
	// If the prompt is asking to implement the primes generator (but isn't the TPM planning phase)
	// We check for keywords related to the task but exclude TPM roles to avoid false positives on the planning prompt.
	isCodingPrompt := (strings.Contains(lowerPrompt, "id:[primes]") ||
		strings.Contains(lowerPrompt, "generate primes") ||
		strings.Contains(lowerPrompt, "primes.json") ||
		(strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")))

	if isCodingPrompt && !strings.Contains(lowerPrompt, "technical program manager") {
		// Return a JSON response with a bash script to create the file.
		// This simulates the "Coding Agent" output format.
		return `{
  "files": [
    {
      "path": "primes.py",
      "content": "def is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nif __name__ == '__main__':\n    import sys\n    limit = int(sys.argv[1]) if len(sys.argv) > 1 else 100\n    primes = [i for i in range(limit + 1) if is_prime(i)]\n    print(primes)"
    }
  ],
  "commands": [
    "python3 primes.py 50"
  ]
}`, nil
	}

	// Default fallback response
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
	return s[:maxLen] + "..."
}
