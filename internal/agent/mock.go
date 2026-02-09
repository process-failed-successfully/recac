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

	upperPrompt := strings.ToUpper(prompt)

	// --- QA AGENT ---
	if strings.Contains(upperPrompt, "QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// --- MANAGER REVIEW ---
	if strings.Contains(upperPrompt, "ROLE - PROJECT MANAGER") || strings.Contains(upperPrompt, "MANAGER REVIEW") {
		return "Project looks good. Approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// --- TPM (Jira Generation) ---
	if strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER") || strings.Contains(upperPrompt, "TPM") {
		return `[
  {
    "title": "ID:[PROJECT] Project Implementation",
    "description": "Implementation of the project requirements",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[TASK-1] Initial Setup",
        "description": "Setup project structure",
        "type": "Task",
        "children": []
      }
    ]
  }
]`, nil
	}

    // --- INITIALIZER ---
    if strings.Contains(upperPrompt, "INITIALIZER") || strings.Contains(upperPrompt, "GET YOUR BEARINGS") || strings.Contains(upperPrompt, "CREATE FEATURE_LIST.JSON") {
         featureList := `{
  "features": [
    {
      "id": "req-setup-repo",
      "title": "Setup Repository",
      "description": "Initialize the repository",
      "status": "todo",
      "type": "task",
      "dependencies": {"depends_on_ids": []}
    },
    {
      "id": "req-implement-primes",
      "title": "Implement Primes",
      "description": "Implement prime number generator",
      "status": "todo",
      "type": "task",
      "dependencies": {"depends_on_ids": ["req-setup-repo"]}
    },
    {
      "id": "req-implement-tests",
      "title": "Implement Tests",
      "description": "Test prime number generator",
      "status": "todo",
      "type": "task",
      "dependencies": {"depends_on_ids": ["req-implement-primes"]}
    },
    {
        "id": "req-the-makefile-targets-are-implemented",
        "title": "Makefile",
        "description": "Create Makefile",
        "status": "todo",
        "type": "task",
        "dependencies": {"depends_on_ids": []}
    },
    {
        "id": "req-ci-workflow",
        "title": "CI Workflow",
        "description": "Create CI workflow",
        "status": "todo",
        "type": "task",
        "dependencies": {"depends_on_ids": []}
    }
  ]
}`
        return fmt.Sprintf("```bash\ncat <<EOF > feature_list.json\n%s\nEOF\nagent-bridge import < feature_list.json\n```", featureList), nil
    }

	// --- CODING AGENT ---
    if strings.Contains(prompt, "req-setup-repo") {
        return "```bash\n# Setup repo\necho 'Setting up repo'\nagent-bridge feature set req-setup-repo --status done --passes true\n```", nil
    }

    if strings.Contains(prompt, "req-implement-primes") {
        pyScript := `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(100) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
print("Generated primes.json")
`
        return fmt.Sprintf("```bash\ncat <<EOF > primes.py\n%s\nEOF\npython3 primes.py\ngit add primes.py primes.json\ngit commit -m 'Implement primes'\nagent-bridge feature set req-implement-primes --status done --passes true\n```", pyScript), nil
    }

    if strings.Contains(prompt, "req-implement-tests") {
        return "```bash\ntouch test_primes.py\nagent-bridge feature set req-implement-tests --status done --passes true\n```", nil
    }

    if strings.Contains(prompt, "req-the-makefile-targets-are-implemented") {
         return "```bash\ntouch Makefile\nagent-bridge feature set req-the-makefile-targets-are-implemented --status done --passes true\n```", nil
    }

    if strings.Contains(prompt, "req-ci-workflow") {
         return "```bash\nmkdir -p .github/workflows\ntouch .github/workflows/ci.yml\nagent-bridge feature set req-ci-workflow --status done --passes true\n```", nil
    }

    // Loop breaker
    if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
        return "```bash\nagent-bridge signal COMPLETED true\n```", nil
    }

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
