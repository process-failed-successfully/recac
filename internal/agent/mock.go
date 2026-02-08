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

	// 1. Loop Breaker Heuristic
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return "Great! Everything looks clean. I will signal success.\n\n```bash\nagent-bridge signal QA_PASSED true\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// 2. TPM Heuristic
	// We check for TPM role and [PRIMES] keyword.
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "[PRIMES]") {
		jsonPlan := `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "type": "Task",
    "description": "Implement primes.py. Repo: <repo_url>",
    "children": [
      {
        "title": "Implement primes.py",
        "type": "Story",
        "description": "Write a python script to calculate primes up to 10000. Repo: <repo_url>",
        "acceptance_criteria": [
          "primes.py exists",
          "contains exactly 1229 primes"
        ]
      }
    ]
  }
]`
		return fmt.Sprintf("Here is the plan:\n```json\n%s\n```", jsonPlan), nil
	}

	// 3. Initializer Heuristic
	// We check for Initializer role but exclude Coding Agent (as coding agent prompts might contain context from initializer)
	if strings.Contains(prompt, "INITIALIZER AGENT") && !strings.Contains(prompt, "CODING AGENT") {
		featureList := `{
  "project_name": "primes-python",
  "features": [
    {
      "id": "req-primes",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement prime number calculator",
      "status": "pending",
      "steps": ["Run primes.py"],
      "passes": false,
      "dependencies": {"depends_on_ids": []}
    },
    {
      "id": "req-primes-py-exists",
      "category": "functional",
      "priority": "MVP",
      "description": "Verify primes.py exists",
      "status": "pending",
      "steps": ["ls primes.py"],
      "passes": false,
      "dependencies": {"depends_on_ids": ["req-primes"]}
    },
    {
      "id": "req-primes-json-exists-and-contain",
      "category": "functional",
      "priority": "MVP",
      "description": "Verify output json exists",
      "status": "pending",
      "steps": ["ls primes.json"],
      "passes": false,
      "dependencies": {"depends_on_ids": ["req-primes"]}
    },
    {
      "id": "req-primes-json-contains-exactly-1",
      "category": "functional",
      "priority": "MVP",
      "description": "Verify correct count",
      "status": "pending",
      "steps": ["grep 1229 primes.json"],
      "passes": false,
      "dependencies": {"depends_on_ids": ["req-primes"]}
    }
  ]
}`
		// Note: we wrap it in cat <<EOF to write to file safely.
		// Using heredoc allows embedded quotes.
		return fmt.Sprintf("I will initialize the project.\n\n```bash\ncat <<EOF > feature_list.json\n%s\nEOF\n```", featureList), nil
	}

	// 4. Coding Agent Heuristic
	// If asked to implement the primes feature
	if strings.Contains(prompt, "CODING AGENT") && (strings.Contains(prompt, "req-primes") || strings.Contains(prompt, "[PRIMES]")) {
		script := `#!/usr/bin/env python3
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(10000) if is_prime(n)]
print(f"Found {len(primes)} primes")

with open("primes.json", "w") as f:
    json.dump({"count": len(primes), "primes": primes}, f)
`
		response := fmt.Sprintf(`I will implement the prime number script.

%[1]sbash
cat <<EOF > primes.py
%[2]s
EOF

# Run it to verify
python3 primes.py

# Mark features as done
agent-bridge feature set req-primes --status done
agent-bridge feature set req-primes-py-exists --status done
agent-bridge feature set req-primes-json-exists-and-contain --status done
agent-bridge feature set req-primes-json-contains-exactly-1 --status done

# Commit
git add .
git commit -m "Implement primes.py"
%[1]s`, "```", script)
		return response, nil
	}

	// 5. Fallback
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
