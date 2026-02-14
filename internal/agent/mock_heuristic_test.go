package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test TPM Heuristic
	tpmPrompt := "You are a Technical Program Manager. Please convert this app_spec.txt into Jira tickets."
	resp, err := agent.Send(ctx, tpmPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, `"title": "ID:[PRIMES] Implement Primes in Python"`) {
		t.Errorf("TPM heuristic failed. Expected JSON with Primes task, got:\n%s", resp)
	}

	// 2. Test Primes Coding Heuristic
	primesPrompt := "You are a coding agent. The task is ID:[PRIMES]. Please implement primes.py."
	resp, err = agent.Send(ctx, primesPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "cat <<EOF > primes.py") {
		t.Errorf("Primes heuristic failed. Expected bash script to create primes.py, got:\n%s", resp)
	}
	if !strings.Contains(resp, "git commit") {
		t.Errorf("Primes heuristic failed. Expected git commit, got:\n%s", resp)
	}

	// 3. Test Completion Heuristic (With ID)
	completionPrompt := "ID:[PRIMES]. git status: nothing to commit, working tree clean"
	resp, err = agent.Send(ctx, completionPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "Task completed") {
		t.Errorf("Completion heuristic failed. Expected 'Task completed', got:\n%s", resp)
	}

	// 4. Test Completion Heuristic (Without ID - Bare Git Status)
	bareCompletionPrompt := "On branch main\nnothing to commit, working tree clean"
	resp, err = agent.Send(ctx, bareCompletionPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "Task completed") {
		t.Errorf("Bare completion heuristic failed. Expected 'Task completed', got:\n%s", resp)
	}

	// 4. Test Default
	defaultPrompt := "Hello world"
	resp, err = agent.Send(ctx, defaultPrompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "In mock mode, I would process this request") {
		t.Errorf("Default heuristic failed. Got:\n%s", resp)
	}
}
