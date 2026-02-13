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
// It returns a mock response based on the prompt content (heuristics)
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for different agent roles

	// 1. TPM Agent (Ticket Generation)
	if strings.Contains(prompt, "Technical Program Manager (TPM)") {
		// Extract Repo URL from prompt to ensure consistency
		repoURL := "https://github.com/example/repo"
		reRepo := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		if matches := reRepo.FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers up to 100. The script should be efficient and well-documented. Repo: %s",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-SCRIPT] Implement Primes Script",
        "description": "Write a Python script named primes.py that calculates primes and outputs them to primes.json. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "primes.json is created",
          "primes.json contains valid prime numbers"
        ],
        "blocked_by": []
      }
    ]
  }
]`, repoURL, repoURL), nil
	}

	// 2. Initializer Agent (Feature Import)
	if strings.Contains(prompt, "Initializer Agent") || strings.Contains(prompt, "INITIALIZER AGENT") {
		// Return a script that pipes JSON to agent-bridge import
		// We use a dummy ID that matches the TPM ticket generation logic if possible,
		// but for the smoke test "prime-python", we just need valid JSON.
		// Note: agent-bridge import expects {"features": [...]}
		return "```bash\n" +
			`echo '{"features": [{"id": "primes-script", "name": "Implement Primes Script", "type": "Story", "status": "todo", "project": "PRIMES"}]}' | agent-bridge import --project "$RECAC_PROJECT_ID"` +
			"\n```", nil
	}

	// 3. Coding Agent (Implementation)
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") || strings.Contains(prompt, "prime") || strings.Contains(prompt, "python") {
		// Check if we already committed (to avoid infinite loop)
		if strings.Contains(prompt, "Add primes script") && (strings.Contains(prompt, "git commit") || strings.Contains(prompt, "Success")) {
			// Work is done. Signal completion via agent-bridge.
			// Ensure the feature exists before setting status (idempotent import)
			return "```bash\n" +
				`echo '{"features": [{"id": "primes-script", "name": "Implement Primes Script", "type": "Story", "status": "completed", "project": "PRIMES"}]}' | agent-bridge import --project "$RECAC_PROJECT_ID" && ` +
				`agent-bridge feature set primes-script --status completed --passes true --project "$RECAC_PROJECT_ID" && ` +
				`agent-bridge feature set req-script-runs-without-errors --status completed --passes true --project "$RECAC_PROJECT_ID" || true && ` +
				`agent-bridge feature set req-primes-json-is-created --status completed --passes true --project "$RECAC_PROJECT_ID" || true && ` +
				`agent-bridge feature set req-primes-json-contains-valid-prime-numbers --status completed --passes true --project "$RECAC_PROJECT_ID" || true && ` +
				`agent-bridge feature set req-primes-json-contains-valid-pri --status completed --passes true --project "$RECAC_PROJECT_ID" || true && ` +
				`agent-bridge signal PROJECT_SIGNED_OFF true --privileged` +
				"\n```\nTask Completed.", nil
		}

		return "```bash\n" +
			`# Generate primes.py
cat <<EOF > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(100) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Run it
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes script"
` + "\n```", nil
	}

	// 4. QA Agent / Manager Review
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "REVIEW") || strings.Contains(prompt, "VERIFY") || strings.Contains(prompt, "Approve or Reject") {
		return "```bash\n" +
			`agent-bridge signal QA_PASSED true --privileged` +
			"\n```\nLGTM. The code looks correct and meets the requirements.", nil
	}

	// Default fallback
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
