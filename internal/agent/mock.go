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

	// Heuristic for Architect/TPM JSON requests
	// The TPM prompt asks for "Output purely JSON" and mentions "Technical Program Manager"
	// The Architect prompt also asks for JSON schemas.
	if (strings.Contains(lowerPrompt, "json") || strings.Contains(lowerPrompt, "schema")) &&
		(strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "output purely json")) {

		// Return a valid JSON structure for tickets
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a Python script to check for prime numbers.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Implement is_prime function",
        "description": "Create a function that returns true if a number is prime.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Function returns true for 2, 3, 5, 7",
          "Function returns false for 4, 6, 8, 9",
          "Function handles edge cases like 0 and 1"
        ],
        "blocked_by": []
      },
      {
        "title": "Create main execution script",
        "description": "Create a script that uses the function to print primes.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Script prints primes up to 100"
        ],
        "blocked_by": ["Implement is_prime function"]
      }
    ]
  }
]`, nil
	}

	// Heuristic for Initializer/Planner JSON requests
	if strings.Contains(lowerPrompt, "planner") || strings.Contains(lowerPrompt, "feature_list.json") {
		return `{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "feature-1",
      "category": "functional",
      "description": "Implement basic functionality",
      "status": "pending",
      "steps": ["Create file", "Run test"],
      "dependencies": {}
    }
  ]
}`, nil
	}

	// Heuristic for Git Lead / Branch Management
	// This prevents no-op loops if the workflow specifically asks for branch setup
	if strings.Contains(lowerPrompt, "git lead") || strings.Contains(lowerPrompt, "create a branch") {
		return `I will ensure the feature branch is ready.

` + "```bash" + `
# Branch creation is handled by SetupWorkspace, but ensure we are on a valid branch
current_branch=$(git rev-parse --abbrev-ref HEAD)
echo "Current branch: $current_branch"
if [ "$current_branch" = "HEAD" ]; then
  git checkout -b feature/primes
fi
` + "```", nil
	}

	// Heuristic for Coding/Implementation Phase (specifically for the Primes scenario)
	// This generates the python file requested in the smoke test
	if strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(lowerPrompt, "prime number script") || (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) {
		return `I will implement the prime number checking script as requested.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    print("Primes up to 100:")
    for i in range(101):
        if is_prime(i):
            print(i)
EOF

# Ensure app_spec.txt exists (guardrail)
if [ ! -f app_spec.txt ]; then
    echo "Prime Number Checker" > app_spec.txt
fi
` + "```", nil
	}

	// Default plain text response
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
