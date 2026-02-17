package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// Heuristic: Check if this is an Execution Phase prompt (Agent Role)
	// If features are pending, we update them. If done, we signal completion.
	if strings.Contains(prompt, `"status": "pending"`) {
		// Special handling for PRIMES task
		// Case-insensitive check to be robust against prompt variations
		promptLower := strings.ToLower(prompt)
		if strings.Contains(promptLower, "primes") || strings.Contains(prompt, "ID:[PRIMES]") {
			return `Here is a plan to implement the primes service.

` + "```bash" + `
#!/bin/bash
# Configure git for mock environment
git config --global user.email "mock@agent.com"
git config --global user.name "Mock Agent"

# Implement primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(2, 10000) if is_prime(p)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Generate the output file
python3 primes.py

# Mark features as done
if [ -f feature_list.json ]; then
  sed -i 's/"status": "pending"/"status": "done"/g' feature_list.json
fi

# Commit changes
git add primes.py primes.json feature_list.json
git commit -m "Implement primes logic" || echo "Nothing to commit"
echo "Success: Mock command executed"
` + "```" + `
`, nil
		}

		return `Here is a plan to implement the pending features.

` + "```bash" + `
#!/bin/bash
# Configure git for mock environment
git config --global user.email "mock@agent.com"
git config --global user.name "Mock Agent"

if [ -f feature_list.json ]; then
  sed -i 's/"status": "pending"/"status": "done"/g' feature_list.json
  git add feature_list.json
  git commit -m "Update feature status" || echo "Nothing to commit"
  echo "Success: Mock command executed"
else
  echo "Error: feature_list.json not found"
fi
` + "```" + `
`, nil
	}

	// Heuristic: Check if this is a Planning Phase prompt (TPM Agent)
	// We check for keywords that appear in the TPM prompt or the expected output format.
	if contains(prompt, "Technical Program Manager") || contains(prompt, "ID:[PRIMES]") {
		// Return a predefined JSON response compatible with the CLI's expectations
		// Extract repo URL from prompt if possible to make it more realistic
		repoURL := "https://github.com/example/repo"
		if matches := regexp.MustCompile(`(?i)Repo: (https?://\S+)`).FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Primes Service",
    "description": "Implement a service that calculates prime numbers. Repo: %s",
    "type": "Epic",
    "acceptance_criteria": [
      "Service calculates primes correctly",
      "API returns JSON"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-API] Implement API",
        "description": "Implement the HTTP API for the primes service. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "GET /primes/{n} returns prime check",
          "Returns 200 OK"
        ]
      }
    ]
  }
]`, repoURL, repoURL), nil
	}

	if strings.Contains(prompt, `"status": "done"`) || strings.Contains(prompt, "All features") {
		return "Task completed. All features implemented.", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func contains(s, substr string) bool {
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
