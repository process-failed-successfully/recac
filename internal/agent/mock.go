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

	// 1. Manager Agent (Sign-off)
	if strings.Contains(prompt, "Manager") && (strings.Contains(prompt, "review") || strings.Contains(prompt, "sign-off")) {
		return "I approve the project.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// 2. QA Agent (Pass)
	if strings.Contains(prompt, "QA") && (strings.Contains(prompt, "verification") || strings.Contains(prompt, "quality assurance")) {
		return "QA passed.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 3. Coding Agent (Completion)
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "software engineer")) && strings.Contains(prompt, "All features are marked as done/passing") {
		return "All features are done.\n```bash\nagent-bridge signal COMPLETED true\n```", nil
	}

	// 4. Coding Agent (Implementation - PRIMES)
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "software engineer")) && (strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py")) {
		script := `import json

primes = []
for num in range(2, 10000):
    is_prime = True
    for i in range(2, int(num ** 0.5) + 1):
        if num % i == 0:
            is_prime = False
            break
    if is_prime:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
`
		return fmt.Sprintf("I will implement the primes script.\n```bash\ncat << 'EOF' > primes.py\n%s\nEOF\npython3 primes.py\ngit add primes.py primes.json\ngit commit -m \"Add primes script and output\"\nagent-bridge feature set PRIMES --status done --passes true\n```", script), nil
	}

	// 5. Initializer / TPM (Feature List)

	// A. Ticket Generation (JSON Output) - Used by recac CLI
	isTicketGen := (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Ticket Generation")) && !strings.Contains(prompt, "CODING AGENT")
	if isTicketGen {
		// Return pure JSON for the CLI to parse
		return `[
  {
    "id": "PRIMES",
    "summary": "Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python. It must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.",
    "type": "Task"
  }
]`, nil
	}

	// B. Feature List File Creation (Bash Output) - Used by Coding Agent
	isInitializer := (strings.Contains(prompt, "feature_list.json") || strings.Contains(prompt, "[PRIMES]")) && !strings.Contains(prompt, "CODING AGENT")

	if isInitializer {
		jsonContent := `{
  "project_name": "primes-project",
  "features": [
    {
      "id": "PRIMES",
      "description": "Calculate primes < 10000",
      "dependencies": {
          "depends_on_ids": []
      },
      "status": "todo"
    }
  ]
}`
		return fmt.Sprintf("I will generate the feature list.\n```bash\ncat <<EOF > feature_list.json\n%s\nEOF\n```", jsonContent), nil
	}

	// Default echo
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
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
