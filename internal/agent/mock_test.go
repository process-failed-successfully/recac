package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. TPM (JSON)
	promptTPM := "You are a Technical Program Manager. Please generate a JSON list of tickets."
	resp, _ := agent.Send(ctx, promptTPM)
	if !strings.Contains(resp, "[") || !strings.Contains(resp, "summary") {
		t.Errorf("Expected JSON response for TPM prompt, got: %s", resp)
	}

	// 2. Git Lead
	promptGit := "I am the Git Lead. Please start the task."
	resp, _ = agent.Send(ctx, promptGit)
	if !strings.Contains(resp, "git checkout -b") {
		t.Errorf("Expected git command for Git Lead prompt, got: %s", resp)
	}

	// 3. Coding (Primes)
	promptCode := "Task ID:[PRIMES]. Please generate primes.py in python."
	resp, _ = agent.Send(ctx, promptCode)
	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Expected bash script to create primes.py, got: %s", resp)
	}

	// 4. Default
	promptDefault := "Hello"
	resp, _ = agent.Send(ctx, promptDefault)
	if !strings.Contains(resp, "Mock agent response") {
		t.Errorf("Expected default response, got: %s", resp)
	}
}
