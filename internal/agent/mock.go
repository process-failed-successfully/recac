package agent

import (
	"context"
	"fmt"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent(model, project string) *MockAgent {
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
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys

	// Heuristic to detect TPM / Planner prompt requesting JSON tickets
	if isTPMPrompt(prompt) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator System",
    "description": "Implementation of prime number generator system.\n\nRepo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-1] Implement Core Generator",
        "description": "Implement the core logic for generating prime numbers.\n\nRepo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": [
          "Must correctly identify prime numbers",
          "Must print primes up to 20"
        ]
      }
    ]
  }
]`, nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isTPMPrompt(prompt string) bool {
	// Check for keywords common in TPM/Planner prompts
	keywords := []string{
		"Technical Program Manager",
		"ticket plan",
		"generate tickets",
		"app_spec",
		"JSON list of tickets",
	}
	for _, kw := range keywords {
		if containsIgnoreCase(prompt, kw) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	// Use stdlib logic by just importing strings if we could, but let's just use strings package since it's already imported
	// Wait, strings is NOT imported in the truncated view I saw earlier?
	// I'll assume I can add it or write a simple one. The file header wasn't fully visible but usually mock.go imports fmt.
	// Let's rely on adding "strings" to imports if needed, but since I can't see the top, I'll write a safe one or assume `strings` is available if I added it.
	// Actually, I can just use a simple loop implementation without `strings` package dependency if I want to be safe,
	// OR I can use `replace_with_git_merge_diff` to add the import.
	// Checking the file content from earlier... "package agent\n\nimport (\n\t"context"\n\t"fmt"\n)"
	// So `strings` is NOT imported. I should implement a simple case-insensitive contains.

	if len(substr) > len(s) {
		return false
	}

	// Convert both to lower case manually for a simple ascii check (sufficient for these keywords)
	sLower := toLowerAscii(s)
	subLower := toLowerAscii(substr)

	for i := 0; i <= len(sLower)-len(subLower); i++ {
		if sLower[i:i+len(subLower)] == subLower {
			return true
		}
	}
	return false
}

func toLowerAscii(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
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
