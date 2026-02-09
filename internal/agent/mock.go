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
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for Smoke Test (E2E)

	// 1. Initializer Agent
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return `
cat <<EOF > feature_list.json
[
  {
    "id": "req-script-prints-primes",
    "name": "Implement Prime Number Script",
    "description": "Create a python script to calculate primes < 10000",
    "priority": "high"
  }
]
EOF
agent-bridge import feature_list.json || echo "Import failed"
`, nil
	}

	// 2. Technical Program Manager (TPM)
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "id": "PRIMES",
    "summary": "Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that outputs primes to 'primes.json'.",
    "type": "Task",
    "status": "todo",
    "priority": "high"
  }
]`, nil
	}

	// 3. Coding Agent
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// Loop Breaker: If git status is clean, we might be done
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return "agent-bridge signal --privileged QA_PASSED true", nil
		}

		// Implement Primes Task
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "req-script-prints-primes") {
			script := `
import json

primes = []
for num in range(2, 10000):
    for i in range(2, int(num**0.5) + 1):
        if (num % i) == 0:
            break
    else:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
print(f"Generated {len(primes)} primes")
`
			// Construct Bash commands
			return fmt.Sprintf(`
cat << 'EOF' > primes.py
%s
EOF

python3 primes.py
git add primes.py primes.json
git diff --cached --quiet || git commit -m "Implement primes script"
agent-bridge signal --privileged QA_PASSED true
`, script), nil
		}

		// Default Coding Response
		return "echo 'Mock Agent: No specific task identified, but I am ready.'", nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "agent-bridge signal --privileged QA_PASSED true", nil
	}

	// 5. Project Manager
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return "agent-bridge signal --privileged PROJECT_SIGNED_OFF true", nil
	}

	// Default response
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

// SendImage implements the VisionAgent interface
func (m *MockAgent) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}
	response := fmt.Sprintf("%s (Vision):\n\nI received your prompt (%d characters) and image (%s). In mock mode, I would analyze the image and text to provide a response.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), imagePath, truncateString(prompt, 100))
	return response, nil
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
