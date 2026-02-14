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

	// TPM / Planning heuristic
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Extract ticket ID from prompt if possible, or use generic
		id := "MFLP-12765" // Default fallback from logs
		if strings.Contains(prompt, "ID:[") {
			start := strings.Index(prompt, "ID:[") + 4
			end := strings.Index(prompt[start:], "]")
			if end != -1 {
				id = prompt[start : start+end]
			}
		} else if strings.Contains(prompt, "ticket_id") {
			// Try to find MFLP-XXXX
			// Fallback
		}

		return fmt.Sprintf(`[{"title": "ID:[%s] Implement Primes in Python", "type": "Task", "description": "Implement a script that prints prime numbers up to 100."}]`, id), nil
	}

	// Primes Scenario heuristic
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		// Check if done
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			if strings.Contains(prompt, "primes.py") {
				// We see the file and git is clean, so we are done.
				// Signal completion via agent-bridge feature set
				// But first verify if we need to import?
				// Memory says: "Mock Agent's completion script ... explicitly imports the task via echo '[...]' | agent-bridge import before attempting to modify its status"
				// Let's be safe and just run python and echo success.
				return "```bash\npython3 primes.py\necho 'Done'\n```", nil
			}
		}

		// Otherwise create file
		return "```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nfor i in range(100):\n    if is_prime(i):\n        print(i)\nEOF\n\ngit add primes.py\ngit commit -m 'Add primes.py'\n```", nil
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
