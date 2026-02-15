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
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// Heuristic to detect TPM Planning Phase
	if len(prompt) > 20 && (contains(prompt, "Technical Program Manager") || contains(prompt, "TPM") || contains(prompt, "architect")) && contains(prompt, "json") {
		// Return a valid JSON response for the planning phase
		return `[
  {
    "id": "TASK-1",
    "title": "Implement Primes Generator",
    "description": "Create a Python script that generates prime numbers.",
    "status": "todo",
    "type": "task",
    "dependencies": []
  }
]`, nil
	}

	// Heuristic for Coding Phase (primes.python scenario)
	if contains(prompt, "primes.json") || contains(prompt, "generate primes") || (contains(prompt, "prime") && contains(prompt, "python")) {
		return "```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nimport json\nprimes = [x for x in range(2, 100) if is_prime(x)]\nwith open('primes.json', 'w') as f:\n    json.dump(primes, f)\nEOF\npython3 primes.py\n```", nil
	}

	// Heuristic for Git Commit Phase
	if contains(prompt, "commit message") {
		return "feat: Implement primes.py", nil
	}


	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(substr) > 0 &&
	       (s == substr ||
	       (len(s) > len(substr) && s[0:len(substr)] == substr) ||
	       (len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
	       func() bool {
		       for i := 0; i <= len(s)-len(substr); i++ {
			       if s[i:i+len(substr)] == substr {
				       return true
			       }
		       }
		       return false
	       }())
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
