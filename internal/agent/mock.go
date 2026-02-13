package agent

import (
	"context"
	"fmt"
	"strings"
	"regexp"
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

	// Heuristic for Initializer (checks for explicit initialization keywords)
	if (strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "initialize")) && strings.Contains(lowerPrompt, "feature") {
		return `I will initialize the project features.

'''bash
echo '{"features":[{"id":"feat-1","category":"core","description":"Implement primes.py","status":"pending","steps":["Create primes.py"],"dependencies":{"depends_on_ids":[],"exclusive_write_paths":[],"read_only_paths":[]}}]}' | agent-bridge import
'''
`, nil
	}

	// Heuristic for 'TPM' (Technical Program Manager) / Ticket Generation
	// This prompt usually contains "Technical Program Manager" or asks for JSON output of Epics/Stories.
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate jira tickets") {

		// Extract Repo URL if present in prompt (usually "Repo: <url>" in description part of spec)
		repoURL := "https://github.com/example/repo" // Default
		// Simple regex to find a URL in the prompt to mimic real agent picking it up from context
		re := regexp.MustCompile(`(?i)(?:repo|repository):\s*(https?://\S+)`)
		match := re.FindStringSubmatch(prompt)
		if len(match) > 1 {
			repoURL = match[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "Prime Number Project",
    "description": "Implement a Python script that calculates prime numbers. Repo: %s",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES] Implement Primes Script",
        "description": "Create a python script that prints the first 10000 prime numbers. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Output matches expected primes"
        ],
        "blocked_by": []
      }
    ]
  }
]`, repoURL, repoURL), nil
	}

	// Heuristic for 'prime-python' coding scenario (Execution Phase)
	// Only trigger if it looks like a coding task and NOT a planning task.
	// The prompt often contains "primes.py" or "prime numbers" AND instructions like "implement" or "write code".
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
