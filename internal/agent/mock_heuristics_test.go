package agent

import (
	"context"
	"recac/internal/agent/prompts"
	"strings"
	"testing"
)

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Test Initializer Heuristic
	t.Run("Initializer", func(t *testing.T) {
		// Load template
		prompt, err := prompts.GetPrompt(prompts.Initializer, map[string]string{
			"spec": "ID:[PRIMES] Prime Number Script",
		})
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		// Send to Mock Agent
		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		// Check if it returned the feature list script
		if !strings.Contains(resp, "agent-bridge import") {
			t.Errorf("Expected Initializer heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})

	// 2. Test TPM Heuristic
	t.Run("TPM", func(t *testing.T) {
		prompt, err := prompts.GetPrompt(prompts.TPMAgent, map[string]string{
			"spec": "ID:[PRIMES] Prime Number Script\nRepo: https://github.com/test/repo",
		})
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		if !strings.Contains(resp, `"title": "ID:[PRIMES] Prime Number Script"`) {
			t.Errorf("Expected TPM heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})

	// 3. Test Coding Agent Heuristic (Implementation)
	t.Run("CodingAgent_Implement", func(t *testing.T) {
		prompt, err := prompts.GetPrompt(prompts.CodingAgent, map[string]string{
			"task_id": "req-primes",
			"task_description": "Implement primes.py to generate primes.json",
			"exclusive_paths": "primes.py",
			"read_only_paths": "none",
			"history": "",
		})
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
			t.Errorf("Expected Coding Agent heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})

	// 4. Test Coding Agent Heuristic (Completion)
	t.Run("CodingAgent_Complete", func(t *testing.T) {
		prompt, err := prompts.GetPrompt(prompts.CodingAgent, map[string]string{
			"task_id": "NONE_ALL_COMPLETE",
			"task_description": "All features are marked as done/passing. Please run final verification and signal completion.",
			"exclusive_paths": "none",
			"read_only_paths": "all",
			"history": "",
		})
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		if !strings.Contains(resp, "agent-bridge signal COMPLETED true") {
			t.Errorf("Expected Completion heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})

	// 5. Test QA Agent Heuristic
	t.Run("QAAgent", func(t *testing.T) {
		prompt, err := prompts.GetPrompt(prompts.QAAgent, nil)
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		if !strings.Contains(resp, "agent-bridge signal QA_PASSED true") {
			t.Errorf("Expected QA heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})

	// 6. Test Manager Agent Heuristic
	t.Run("ManagerAgent", func(t *testing.T) {
		prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
			"qa_report": "QA_PASSED=true",
		})
		if err != nil {
			t.Fatalf("Failed to load prompt: %v", err)
		}

		resp, err := agent.Send(ctx, prompt)
		if err != nil {
			t.Fatalf("Failed to send prompt: %v", err)
		}

		if !strings.Contains(resp, "agent-bridge signal PROJECT_SIGNED_OFF true") {
			t.Errorf("Expected Manager heuristic to trigger, got default response.\nPrompt preview: %s\nResponse: %s", prompt[:100], resp)
		}
	})
}
