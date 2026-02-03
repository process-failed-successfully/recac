package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. Scenario Generation (TPM Agent)
	// Triggers: "generate-from-spec", "technical program manager"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate-from-spec") {
		// Return JSON array of ticket nodes
		return `[
  {
    "title": "Implement Primes",
    "description": "Create a python script that calculates primes",
    "type": "Story",
    "acceptance_criteria": [
      "The list of primes in 'primes.json' contains exactly 1229 primes"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Initializer (Feature List)
	// Triggers: "initializer agent" AND "feature_list.json"
	if strings.Contains(lowerPrompt, "initializer agent") && strings.Contains(lowerPrompt, "feature_list.json") {
		// Return script to create feature_list.json
		return "```bash\n" + `
echo '{
  "req-the-list-of-primes-in-primes-j": {
    "id": "req-the-list-of-primes-in-primes-j",
    "description": "The list of primes in primes.json contains exactly 1229 primes",
    "files": ["primes.json"],
    "content": "1229"
  }
}' > feature_list.json
echo "Initialized feature list"
` + "\n```\n", nil
	}

	// 3. Implementation (Coder Agent)
	// Triggers: "primes", "python" (specific to prime-python scenario)
	if strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "python") {
		return "```bash\n" + `
# Configure git
git config --global user.email "mock@example.com"
git config --global user.name "Mock Agent"

# Create primes.py
cat << 'EOF' > primes.py
import json

def calculate_primes(n):
    primes = []
    for possiblePrime in range(2, n):
        isPrime = True
        for num in range(2, int(possiblePrime ** 0.5) + 1):
            if possiblePrime % num == 0:
                isPrime = False
                break
        if isPrime:
            primes.append(possiblePrime)
    return primes

def main():
    primes = calculate_primes(10000)
    with open('primes.json', 'w') as f:
        json.dump({'primes': primes}, f)

if __name__ == "__main__":
    main()
EOF

# Run it
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement primes" || echo "Nothing to commit"

# Signal completion
if command -v agent-bridge &> /dev/null; then
    agent-bridge feature set req-the-list-of-primes-in-primes-j --status done --passes true
fi
` + "\n```\n", nil
	}

	// 4. QA / Manager
	if strings.Contains(lowerPrompt, "qa agent") {
		return "```bash\nif command -v agent-bridge &> /dev/null; then\n    agent-bridge signal QA_PASSED true\nfi\n```\nApproved. QA Passed.", nil
	}

	if strings.Contains(lowerPrompt, "project manager") {
		return "```bash\nif command -v agent-bridge &> /dev/null; then\n    agent-bridge signal PROJECT_SIGNED_OFF true\nfi\n```\nApproved. Project Signed Off.", nil
	}

	// Default fallback
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
