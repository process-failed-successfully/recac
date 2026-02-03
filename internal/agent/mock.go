package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var repoRegex = regexp.MustCompile(`Repo: (https://[^\s]+)`)

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

	// 1. TPM / Ticket Generation Heuristic
	if isTPMPrompt(prompt) {
		// Extract Repo URL from prompt if present
		repo := "https://github.com/process-failed-successfully/recac"
		if matches := repoRegex.FindStringSubmatch(prompt); len(matches) > 1 {
			repo = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script that generates prime numbers.\nRepo: %s",
    "type": "Story",
    "acceptance_criteria": [
      "Create primes.py",
      "Implement verify_prime function",
      "Add unit tests"
    ],
    "children": []
  }
]`, repo), nil
	}

	// 2. Initializer Heuristic (feature_list.json)
	if containsIgnoreCase(prompt, "Initializer") || containsIgnoreCase(prompt, "feature_list.json") {
		// The environment usually handles feature injection now, but if asked:
		return `I will create the feature list.
` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "Prime Generator",
  "features": [
    {"id": "req-create-primes-py", "description": "Create primes.py", "status": "pending", "priority": "critical"},
    {"id": "req-implement-verify-prime-functio", "description": "Implement verify_prime function", "status": "pending", "priority": "critical"},
    {"id": "req-add-unit-tests", "description": "Add unit tests", "status": "pending", "priority": "critical"}
  ]
}
EOF
cat feature_list.json | agent-bridge import
` + "```" + `
`, nil
	}

	// 3. Manager Heuristic
	if containsIgnoreCase(prompt, "Manager") || containsIgnoreCase(prompt, "Project Manager") {
		return `The project looks good. Approved.
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// 4. QA Heuristic
	if containsIgnoreCase(prompt, "QA Agent") || containsIgnoreCase(prompt, "Quality Assurance") || containsIgnoreCase(prompt, "Verify the project") {
		return `QA Checks Passed.
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 5. Implementation Heuristic (Primes)
	// We look for "primes.py" or "primes" in the prompt to trigger the coding logic
	if containsIgnoreCase(prompt, "primes") || containsIgnoreCase(prompt, "prime number") {
		return `I will implement the prime number generator as requested.
` + "```bash" + `
# Create the implementation
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == "__main__":
    primes = get_primes(10000)
    with open('primes.json', 'w') as f:
        json.dump({"primes": primes}, f)
    print(f"Generated {len(primes)} primes.")
EOF

# Run it to generate the json
python3 primes.py

# Git add
git add primes.py primes.json

# Mark features as done (using ids from injected list)
agent-bridge feature update req-create-primes-py done
agent-bridge feature update req-implement-verify-prime-functio done
agent-bridge feature update req-add-unit-tests done
` + "```" + `
`, nil
	}

	// Default Response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isTPMPrompt(prompt string) bool {
	p := truncateString(prompt, 200) // Optimization
	return (containsIgnoreCase(p, "Technical Program Manager") || containsIgnoreCase(p, "TPM"))
}

func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
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
