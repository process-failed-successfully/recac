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

	// Heuristic for TPM / Architect phase
	// If the prompt asks for a JSON plan (which mentions "Technical Program Manager" or "Architect")
	if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Architect")) && strings.Contains(strings.ToLower(prompt), "json") {
		return `[
  {
    "summary": "Implement Core Features",
    "description": "Implement the core features as requested.",
    "type": "Epic",
    "repo_url": "https://github.com/process-failed-successfully/recac-jira-e2e",
    "children": [
      {
        "summary": "Setup Project Structure",
        "description": "Initialize the project structure.",
        "type": "Story",
        "children": []
      },
      {
        "summary": "Implement Feature A",
        "description": "Implement feature A logic.",
        "type": "Story",
        "children": []
      }
    ]
  }
]`, nil
	}

	// Heuristic for Git Lead phase
	if strings.Contains(prompt, "Git Lead") {
		return "I will create the branch.\n\n```bash\ngit checkout -b feature/setup\n```", nil
	}

	// Heuristic for Coding phase (Prime Python)
	lowerPrompt := strings.ToLower(prompt)
	if (strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(lowerPrompt, "generate primes") || strings.Contains(lowerPrompt, "primes.json") || (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python"))) && !strings.Contains(lowerPrompt, "technical program manager") {
		return "Here is the python script:\n\n```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\nEOF\n```", nil
	}

	// Heuristic for Commit Message phase
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
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
