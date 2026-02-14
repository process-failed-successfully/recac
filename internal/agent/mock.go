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

	// Heuristic: If prompt is for TPM (Planning), return a JSON ticket list
	if contains(prompt, "Technical Program Manager") || contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "type": "Task",
    "description": "Create a python script primes.py that calculates the first 100 prime numbers and saves them to primes.json."
  }
]`, nil
	}

	// Heuristic: If prompt is for the Primes task implementation
	if contains(prompt, "ID:[PRIMES]") || contains(prompt, "primes.py") {
		// Check if we are in the completion phase (e.g. prompt contains git status indicating clean)
		if contains(prompt, "nothing to commit") || contains(prompt, "working tree clean") {
			return `Task completed.`, nil
		}

		// Otherwise, return the implementation script
		return `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
num = 2
while len(primes) < 100:
    if is_prime(num):
        primes.append(num)
    num += 1

with open('primes.json', 'w') as f:
    json.dump(primes, f)
print(f"Generated {len(primes)} primes")
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement prime generator"
`, nil
	}

	// Default response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
	// Simple containment check, could be replaced with strings.Contains if we import strings
	return len(s) >= len(substr) && len(substr) > 0 &&
		(s == substr ||
		(len(s) > len(substr) && (s[0:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr))) ||
		// Fallback to searching (inefficient but fine for mock)
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
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
