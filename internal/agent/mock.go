package agent

import (
	"context"
	"fmt"
	"strings"
	"regexp"
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

	isPlanning := strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket")

	// Heuristic for Planning Phase (Ticket Generation)
	// Check this BEFORE Execution Phase heuristics to prevent false positives when AppSpecs contain target filenames/keywords.
	if isPlanning {
		// Extract repo URL if possible to make tickets look realistic
		repo := "https://github.com/org/repo"
		re := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		if matches := re.FindStringSubmatch(prompt); len(matches) > 1 {
			repo = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement prime number generator",
    "description": "Create a Python script that generates prime numbers up to 10,000.\n\nRepo: %s",
    "type": "Story",
    "acceptance_criteria": [
      "Script name: primes.py",
      "Output file: primes.json"
    ]
  }
]`, repo), nil
	}

	// Heuristic for 'prime-python' scenario (Execution Phase)
	// We check for specific ID (strong signal), OR file/keyword IF it's not a planning request (weak signal).
	// Note: The `!isPlanning` check is redundant now because we handle planning above, but kept for clarity/safety.
	if strings.Contains(prompt, "ID:[PRIMES]") || (strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime")) {
		return `
Sure, here is a Python script that calculates prime numbers up to 10,000 and writes them to a file named 'primes.json'.

filename: primes.py
` + "```python" + `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10001) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)

print(f"Found {len(primes)} prime numbers.")
` + "```" + `
`, nil
	}

	// Heuristic for Project Manager / QA Sign-off
	if strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "qa") {
		if strings.Contains(lowerPrompt, "sign off") || strings.Contains(lowerPrompt, "review") {
			return `
I have reviewed the work and it looks correct.

filename: sign_off.txt
` + "```" + `
PROJECT_SIGNED_OFF
QA_PASSED
` + "```" + `
`, nil
		}
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
