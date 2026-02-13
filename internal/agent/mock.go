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

	// Heuristic for 'prime-python' scenario
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime numbers") {
		return `Here is a Python script to print the first 10000 prime numbers.

'''python
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

count = 0
num = 2
while count < 10000:
    if is_prime(num):
        print(num)
        count += 1
    num += 1
'''

And here are the commands to commit it:

'''bash
git add primes.py
git commit -m "Add primes.py"
'''
`, nil
	}

	// Heuristic for Initializer (if needed by workflow)
	// The memory mentioned: "The `MockAgent` heuristic for the 'Initializer Agent' returns a Bash script piping a JSON object to `agent-bridge import`"
	if strings.Contains(lowerPrompt, "initializer") || strings.Contains(lowerPrompt, "setup project") {
		return `I will initialize the project features.

'''bash
echo '{"features":[{"id":"feat-1","category":"core","description":"Implement primes.py","status":"pending","steps":["Create primes.py"],"dependencies":{"depends_on_ids":[],"exclusive_write_paths":[],"read_only_paths":[]}}]}' | agent-bridge import
'''
`, nil
	}

	// Return a mock response that shows the agent received the prompt
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
