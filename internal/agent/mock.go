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
// It uses heuristics to return appropriate responses for E2E scenarios
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// --- Heuristic 1: TPM / Ticket Generation ---
	// The prompt typically contains "Technical Program Manager" or asks to "create tickets"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "create ticket") || strings.Contains(lowerPrompt, "generate ticket") {
		// Return a JSON list of tickets for the Prime Number scenario
		return `[
  {
    "title": "Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates all primes < 10000 and outputs to 'primes.json'.",
    "type": "Task"
  }
]`, nil
	}

	// --- Heuristic 2: Coding Agent (Prime Number Scenario) ---
	// The prompt asks for python code to calculate primes
	if (strings.Contains(lowerPrompt, "python") && strings.Contains(lowerPrompt, "prime")) || strings.Contains(lowerPrompt, "primes.py") {
		return `Here is the solution.

I will create the python script 'primes.py' that calculates prime numbers up to 10,000 and saves them to 'primes.json'.

```bash
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
print(f"Generated {len(primes)} primes.")
EOF
```

Now I will run the script to generate the JSON file.

```bash
python3 primes.py
```

And finally, I will verify the output exists.

```bash
ls -l primes.json
```
`, nil
	}

	// --- Heuristic 3: Manager / QA ---
	// If asked to verify or review
	if strings.Contains(lowerPrompt, "verify") || strings.Contains(lowerPrompt, "review") || strings.Contains(lowerPrompt, "qa") {
		return "The changes look correct. The code implements the prime number calculation as requested and outputs the correct JSON format. Verified.", nil
	}

	// --- Default Response ---
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
