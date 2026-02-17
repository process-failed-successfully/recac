package agent

import (
	"context"
	"fmt"
	"strings"
	"regexp"
)

// MockAgent is a smart mock agent for testing and mock mode.
// It uses heuristics to return realistic responses for E2E scenarios.
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

	// Heuristics for E2E Smoke Test (Primes)

	// 1. Planning Phase (TPM)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Extract Repo URL if present
		repoURL := "https://github.com/example/repo"
		re := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		match := re.FindStringSubmatch(prompt)
		if len(match) > 1 {
			repoURL = match[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "Implement Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. The script MUST be named 'primes.py'. The output file MUST be named 'primes.json'. Implement prime calculation logic in primes.py, output results to primes.json, validate that the output file contains a 'primes' list, verify that exactly 1229 primes are calculated, and commit primes.json to the repository. Repo: %s",
    "type": "task",
    "priority": "high",
    "dependencies": []
  }
]`, repoURL), nil
	}

	// 2. Coding Phase (Developer)
	// Check for task execution prompt
	if strings.Contains(strings.ToLower(prompt), "implement") || strings.Contains(strings.ToLower(prompt), "write") || strings.Contains(prompt, "req-") {
		// If we see success output, we are done
		if strings.Contains(prompt, "True") || strings.Contains(prompt, "1229") || strings.Contains(prompt, "Feature req-") {
			return "Task completed. The script is implemented and verified.", nil
		}

		// Otherwise, return the implementation script
		// We return a self-contained bash script that does everything:
		// 1. Create primes.py
		// 2. Install deps (if any, none for pure python here usually, but smoke test does apt-get)
		// 3. Run it
		// 4. Verify it
		// 5. Update status
		return ````bash
cat << 'EOF' > primes.py
import json

def calculate_primes(n: int) -> list:
    primes = []
    for possiblePrime in range(2, n + 1):
        isPrime = True
        for num in range(2, int(possiblePrime ** 0.5) + 1):
            if possiblePrime % num == 0:
                isPrime = False
                break
        if isPrime:
            primes.append(possiblePrime)
    return primes

def write_primes_to_file(primes: list, filename: str) -> None:
    with open(filename, 'w') as f:
        json.dump({'primes': primes}, f)

def main():
    n = 10000
    primes = calculate_primes(n)
    write_primes_to_file(primes, 'primes.json')

if __name__ == '__main__':
    main()
EOF

# Run the script
python3 primes.py

# Verify output
python3 -c "import json; primes = json.load(open('primes.json'))['primes']; print(len(primes) == 1229)"

# Update status (Mocking the ID we expect from the plan)
# Note: In real run, the ID is dynamic. But for the single-task smoke test, we can try to guess or just mark the one we see in prompt.
# The prompt usually contains the feature list.
# Let's extract the ID from the prompt if possible, or just print a success message.
# The smoke test looks for "Feature ... updated" or just success.

# For the smoke test specifically, we use a known ID if we can see it in the prompt feature list.
# The prompt has: "id": "req-the-primes-py-script-is-implem" ...
# We'll just mark the first one we find as done.

FEATURE_ID=$(grep -o '"id": "req-[^"]*"' feature_list.json | head -1 | cut -d'"' -f4)
if [ -n "$FEATURE_ID" ]; then
  agent-bridge feature set $FEATURE_ID --status done --passes true
fi

echo "- Implemented prime number calculation feature" >> successes.txt
````, nil
	}

	// 3. Default Fallback
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
