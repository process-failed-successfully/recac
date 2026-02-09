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

	// Heuristic for E2E Tests: [PRIMES] Scenario
	// We check for [PRIMES], primes.py, or prime-python to ensure we catch all relevant contexts
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "prime-python") {
		// 1. Technical Program Manager (Ticket Generation)
		// Detects "Technical Program Manager" role or ticket generation instructions
		if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION")) &&
			!strings.Contains(prompt, "YOUR ROLE - CODING AGENT") {
			return `[
  {
    "title": "[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file (e.g., cat << 'EOF' > primes.py). Do not output raw python code.\nCommit 'primes.py' and 'primes.json' IMMEDIATELY. Use 'git add -f primes.json' to ensure it is tracked.\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: ` + "`" + `{\"primes\": [2, 3, 5, ...]}` + "`" + `.\nIMPORTANT: Ensure the FINAL primes.json committed to the repository contains ALL primes less than 10,000 (Exactly 1229 primes).\nDo not truncate it for testing or reporting - the verification script expects the full list.\nKeep the code absolutely minimal. Finish as quickly as possible.",
    "type": "Task",
    "children": []
  }
]`, nil
		}

		// 2. Initializer (Feature List Generation)
		// Detects "Initializer" role or feature list requests
		if strings.Contains(prompt, "Initialize the project") || strings.Contains(prompt, "feature_list.json") {
			return "```bash\n" +
				"cat << 'EOF' > feature_list.json\n" +
				`{
  "project_name": "prime-python",
  "features": [
    {
      "id": "req-primes-py-exists",
      "description": "Create primes.py script",
      "priority": "1"
    }
  ]
}` + "\nEOF\n```", nil
		}

		// 3. Coding Agent (Implementation)
		// Detects "Coding Agent" role OR generic task execution prompt
		if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") ||
			strings.Contains(prompt, "Implement the solution") ||
			strings.Contains(prompt, "Multiple/Not Assigned") ||
			strings.Contains(prompt, "Create Prime Number Script") {
			return "```bash\n" +
				"cat << 'EOF' > primes.py\n" +
				"import json\n\n" +
				"def is_prime(n):\n" +
				"    if n < 2: return False\n" +
				"    for i in range(2, int(n**0.5) + 1):\n" +
				"        if n % i == 0: return False\n" +
				"    return True\n\n" +
				"primes = [i for i in range(10000) if is_prime(i)]\n" +
				"with open('primes.json', 'w') as f:\n" +
				"    json.dump({'primes': primes}, f)\n" +
				"EOF\n\n" +
				"python3 primes.py\n" +
				"git add primes.py primes.json\n" +
				"git commit -m \"Add primes.py and primes.json\"\n" +
				"```", nil
		}
	}

	// Return a generic mock response that shows the agent received the prompt
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
