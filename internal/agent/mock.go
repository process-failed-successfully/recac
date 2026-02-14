package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// Helper structs for JSON response
type MockTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type MockFeature struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Priority     string                 `json:"priority"`
	Status       string                 `json:"status"`
	Steps        []string               `json:"steps"`
	Dependencies map[string]interface{} `json:"dependencies"`
}

type MockFeatureList struct {
	ProjectName string        `json:"project_name"`
	Features    []MockFeature `json:"features"`
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

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. CLI Ticket Planning Phase (TPM Role)
	// Trigger on "technical program manager" or "generate ticket"
	// This is used by `recac generate-from-spec` which EXPECTS PURE JSON.
	if strings.Contains(lowerPrompt, "technical program manager") ||
		strings.Contains(lowerPrompt, "generate ticket") {

		// Extract repo URL from prompt if possible, or use a placeholder
		repoURL := "https://github.com/example/repo"
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			repoURL = strings.Split(parts[1], "\n")[0]
		}
		repoURL = strings.TrimSpace(repoURL)

		task := MockTask{
			Title:       "ID:[PRIMES] Implement Prime Number Python Script",
			Description: fmt.Sprintf("Create a python script named primes.py that calculates primes up to 10000. \n\nRepo: %s\n\nAppSpec:\nruntime: python\n...", repoURL),
			Type:        "Task",
		}

		jsonData, err := json.MarshalIndent([]MockTask{task}, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal mock task: %w", err)
		}

		return string(jsonData), nil
	}

	// 1.5 Initializer Agent (RunLoop Phase)
	// Trigger on "initializer agent"
	// This runs inside the agent loop and MUST return a bash script to create features.
	if strings.Contains(lowerPrompt, "initializer agent") {
		// Extract repo URL from prompt if possible, or use a placeholder
		repoURL := "https://github.com/example/repo"
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			repoURL = strings.Split(parts[1], "\n")[0]
		}
		repoURL = strings.TrimSpace(repoURL)

		featureList := MockFeatureList{
			ProjectName: "Prime Calculator",
			Features: []MockFeature{
				{
					ID:          "[PRIMES]",
					Title:       "Implement Prime Number Python Script",
					Description: fmt.Sprintf("Create a python script named primes.py that calculates primes up to 10000. \n\nRepo: %s", repoURL),
					Priority:    "MVP",
					Status:      "pending",
					Steps: []string{
						"Create primes.py",
						"Run python3 primes.py",
						"Verify primes.json is created",
					},
					Dependencies: map[string]interface{}{
						"exclusive_write_paths": []string{"primes.py"},
						"read_only_paths":       []string{},
					},
				},
			},
		}

		jsonData, err := json.MarshalIndent(featureList, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal mock feature list: %w", err)
		}

		// Initializer response must use 'agent-bridge import' as per new prompt instructions
		// We return a bash block that writes to file AND pipes to import for robustness.
		return fmt.Sprintf(`Creating feature list.

`+"```bash"+`
cat << 'EOF' > feature_list.json
%s
EOF

# Import to DB (Primary)
cat feature_list.json | agent-bridge import || echo "Import failed, relying on file fallback"
`+"```", string(jsonData)), nil
	}

	// 2. QA Phase
	// Triggers on "QA report" or similar
	// We check for "role - qa agent" specifically to avoid false positives if a coding prompt mentions QA
	if strings.Contains(lowerPrompt, "role - qa agent") || strings.Contains(lowerPrompt, "qa report") {
		return "QA passed.\n\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 3. Manager Review Phase
	// Triggers on "manager agent" or "review"
	if strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "role - manager") {
		return "Project signed off.\n\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
	}

	// 4. Completion Check
	// If the previous command resulted in "nothing to commit" or "working tree clean",
	// it means the task is already done. We should signal completion.
	// We also check for "everything up-to-date" which typically follows "nothing to commit" in git output.
	if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") || strings.Contains(lowerPrompt, "everything up-to-date") {
		return "Great! The work is done. Marking feature as complete.\n\n```bash\nagent-bridge feature set \"[PRIMES]\" --status done --passes true\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
	}

	// 5. Execution Phase (Coding Agent)
	// Prime Python Scenario - triggers when asked to write code
	// We check for "prime" and "python" BUT NOT if we are in the Initializer phase (which also has these words)
	// Note: Initializer prompt now triggers Block 1.5 because of "initializer agent" check order if placed before?
	// Actually, "initializer agent" is checked in Block 1.5.
	// So we keep the exclusion here just in case Block 1.5 conditions are missed but these match.
	if strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python") &&
		!strings.Contains(lowerPrompt, "initializer agent") {

		// Safety Valve: If we somehow missed the completion check above but see indications of completion,
		// trigger completion now to prevent infinite loops.
		if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") {
			fmt.Println("DEBUG: MockAgent Safety Valve Triggered (Loop Prevention)")
			return "Great! The work is done. Marking feature as complete.\n\n```bash\nagent-bridge feature set \"[PRIMES]\" --status done --passes true\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
		}

		// Debug log to help diagnose loop issues in CI
		fmt.Printf("DEBUG: MockAgent Triggering Prime Python Script Generation. Prompt len: %d\n", len(prompt))

		return `I will create a python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
count = 0
for i in range(10000):
    if is_prime(i):
        primes.append(i)
        count += 1
print(f"Found {count} primes")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script"
git push origin HEAD
` + "```", nil
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
