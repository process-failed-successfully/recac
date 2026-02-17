package agent

import (
	"context"
	"encoding/json"
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

	// Handle prime-python scenario
	if strings.Contains(prompt, "primes.py") {
		primesJSON := generatePrimesJSON()
		response := fmt.Sprintf(`cat << 'EOF' > primes.json
%s
EOF

cat << 'EOF' > primes.py
import json

def calculate_primes():
    primes = []
    is_prime = [True] * 10000
    is_prime[0] = is_prime[1] = False

    for p in range(2, int(10000**0.5) + 1):
        if is_prime[p]:
            for i in range(p * p, 10000, p):
                is_prime[i] = False

    for p in range(2, 10000):
        if is_prime[p]:
            primes.append(p)
    return primes

if __name__ == "__main__":
    primes = calculate_primes()
    with open('primes.json', 'w') as f:
        json.dump({"primes": primes}, f)
EOF

git add primes.json primes.py
git commit -m "Add primes implementation"
echo "Task completed."
`, primesJSON)
		return response, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func generatePrimesJSON() string {
	primes := []int{}
	isPrime := make([]bool, 10000)
	for i := range isPrime {
		isPrime[i] = true
	}
	isPrime[0] = false
	isPrime[1] = false

	for p := 2; p*p < 10000; p++ {
		if isPrime[p] {
			for i := p * p; i < 10000; i += p {
				isPrime[i] = false
			}
		}
	}
	for p := 2; p < 10000; p++ {
		if isPrime[p] {
			primes = append(primes, p)
		}
	}

	result := struct {
		Primes []int `json:"primes"`
	}{
		Primes: primes,
	}

	data, _ := json.Marshal(result)
	return string(data)
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
