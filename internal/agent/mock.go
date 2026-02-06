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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Debug logging
	fmt.Printf("DEBUG: MockAgent Prompt: %s\n", truncateString(prompt, 200))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E Smoke Tests

	// 1. Ticket Generation
	if strings.Contains(prompt, "critical instruction for ticket generation") || strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script Task",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Task",
    "labels": ["backend", "python"],
    "acceptance_criteria": [
      "The script 'primes.py' is implemented and calculates all prime numbers less than 10,000",
      "The results are output to a file named 'primes.json' in the correct JSON format",
      "The output file 'primes.json' contains a 'primes' list",
      "Exactly 1229 primes are calculated and included in the 'primes.json' file",
      "The 'primes.json' file is committed to the repository"
    ],
    "dependencies": [],
    "story_points": 3
  }
]`, nil
	}

	// 2. Initializer (Feature Import)
	if strings.Contains(prompt, "feature_list.json") && (strings.Contains(prompt, "INITIALIZER") || strings.Contains(prompt, "Initialize")) {
		return "```bash\necho '[]' > feature_list.json\n```", nil
	}

	// 3. Implementation (Primes)
	// Matches "calculate prime", "calculates prime", "calculates all prime", "[PRIMES]", "primes.py", etc.
	lowerPrompt := strings.ToLower(prompt)
	if (strings.Contains(lowerPrompt, "calculate") && strings.Contains(lowerPrompt, "prime")) ||
		strings.Contains(prompt, "[PRIMES]") ||
		strings.Contains(lowerPrompt, "primes.py") ||
		strings.Contains(lowerPrompt, "primes.json") {

		// Guard: If we already implemented it (in history), signal completion
		if strings.Contains(prompt, "I will implement the prime number calculation script") {
			return "I have already implemented the script. Marking as complete.\n\n" +
				"```bash\n" +
				"agent-bridge signal COMPLETED true\n" +
				"```", nil
		}

		return "I will implement the prime number calculation script.\n\n" +
			"```bash\n" +
			"cat <<EOF > primes.py\n" +
			"import json\n" +
			"\n" +
			"def is_prime(n):\n" +
			"    if n < 2:\n" +
			"        return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n" +
			"\n" +
			"primes = [x for x in range(10000) if is_prime(x)]\n" +
			"\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({\"primes\": primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"```\n" +
			"\n" +
			"COMPLETED\n", nil
	}

	// 4. Generic Bootstrap (Coding Agent)
	// If no other heuristic matched, but we are in the Coding Agent role (identified by the "GET YOUR BEARINGS" step),
	// run the bootstrap commands. This avoids NO-OP loops when the prompt lacks specific task keywords.
	if strings.Contains(prompt, "STEP 1: GET YOUR BEARINGS") {
		return "I will start by orienting myself.\n\n```bash\nls -la\ncat feature_list.json\n```", nil
	}

	// 5. QA Agent
	// 5. QA Agent
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "QA_PASSED\n\n```bash\nagent-bridge signal set QA_PASSED true\n```", nil
	}

	// 6. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "PROJECT_SIGNED_OFF\n\n```bash\nagent-bridge signal set PROJECT_SIGNED_OFF true\n```", nil
	}

	// Default Echo
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
