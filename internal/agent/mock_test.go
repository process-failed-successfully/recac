package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMockAgent_TPM_RepoExtraction(t *testing.T) {
	agent := NewMockAgent()

	// Simulating the prompt from the CI logs
	prompt := `
ID:[PRIMES] Prime Number Script
Repo: https://github.com/example/repo. Use the repository associated with the project.
6. **Blockers**: ...
`
	resp, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Agent.Send failed: %v", err)
	}

	// Try to unmarshal the response. If the repo extraction is buggy, this will fail
	// because literal newlines will be injected into the JSON string.
	var result interface{}
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		t.Logf("Response was:\n%s", resp)
		t.Fatalf("JSON Unmarshal failed (reproduced issue): %v", err)
	}
}
