package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/db"
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

	// Heuristics for E2E tests (specifically prime-python scenario)
	lowerPrompt := strings.ToLower(prompt)

	// 1. Planning Phase / TPM
	// The agent usually asks for a plan first.
	// If the prompt mentions "Technical Program Manager" or asks to "break down" the task.
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "break down") {
		// Extract Repo URL if present
		repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
		matches := repoRegex.FindStringSubmatch(prompt)
		repoSuffix := ""
		if len(matches) > 1 {
			repoSuffix = fmt.Sprintf("\\nRepo: %s", matches[1])
		}

		// Return a JSON plan.
		// Use json.Marshal to ensure validity and proper escaping
		featureList := db.FeatureList{
			ProjectName: "mock-project",
			Features: []db.Feature{
				{
					ID:          "req-primes",
					Description: fmt.Sprintf("Write a python script to calculate primes under 10000.%s", repoSuffix),
					Status:      "pending",
					Priority:    "critical",
					Category:    "functional",
				},
			},
		}

		jsonData, err := json.MarshalIndent(featureList, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal mock plan: %w", err)
		}

		return fmt.Sprintf("I will initialize the project plan.\n\n```bash\ncat << 'EOF' > feature_list.json\n%s\nEOF\n```", string(jsonData)), nil
	}

	// 1.5. All Done / No Task
	if strings.Contains(lowerPrompt, "all features are marked as done") {
		return "Task completed. Standing by.", nil
	}

	// 2. Coding Phase / Implementation
	// If the prompt asks to "implement" or mentions the specific file requirements.
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "implement") {
		// Check for successful execution
		// The runner saves output with role "System", containing "Success: Mock command executed"
		if strings.Contains(lowerPrompt, "success: mock command executed") {
			// Update feature_list.json to mark task as done to progress workflow
			return `Task completed. Marking feature as done.
` + "```bash" + `
sed -i 's/"status": "pending"/"status": "done"/' feature_list.json
` + "```", nil
		}

		return `I will create the python script to calculate prime numbers.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script"
` + "```", nil
	}

	// 3. QA / Manager Sign-off
	if strings.Contains(lowerPrompt, "qa agent") {
		return "QA_PASSED", nil
	}
	if strings.Contains(lowerPrompt, "manager agent") {
		return "PROJECT_SIGNED_OFF", nil
	}

	// Default response
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
