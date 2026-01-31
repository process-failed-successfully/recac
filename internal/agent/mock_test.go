package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_SmokeTestSequence(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Manager Role
	managerPrompt := "You are an expert Technical Product Manager. Here is the app_spec.txt... calculate prime numbers... Ticket Jira..."
	resp1, err := agent.Send(ctx, managerPrompt)
	if err != nil {
		t.Fatalf("Manager prompt failed: %v", err)
	}
	if !strings.Contains(resp1, "ID:[PRIMES]") {
		t.Errorf("Expected Manager response to contain ticket ID, got: %s", resp1)
	}
	if !strings.Contains(resp1, "Epic") {
		t.Errorf("Expected Manager response to contain 'Epic', got: %s", resp1)
	}

	// 2. Agent Role - Step 1
	agentPrompt1 := "You are an expert Software Engineer."
	resp2, err := agent.Send(ctx, agentPrompt1)
	if err != nil {
		t.Fatalf("Agent prompt 1 failed: %v", err)
	}
	if !strings.Contains(resp2, "ls -la") {
		t.Errorf("Expected 'ls -la', got: %s", resp2)
	}

	// 3. Agent Role - Step 2 (Result of ls -la)
	agentPrompt2 := agentPrompt1 + "\n\ncommand output:\nls -la"
	resp3, err := agent.Send(ctx, agentPrompt2)
	if err != nil {
		t.Fatalf("Agent prompt 2 failed: %v", err)
	}
	if !strings.Contains(resp3, "cat app_spec.txt") {
		t.Errorf("Expected 'cat app_spec.txt', got: %s", resp3)
	}

	// 4. Agent Role - Step 3 (Result of cat)
	agentPrompt3 := agentPrompt2 + "\n\ncommand output:\ncat app_spec.txt..."
	resp4, err := agent.Send(ctx, agentPrompt3)
	if err != nil {
		t.Fatalf("Agent prompt 3 failed: %v", err)
	}
	if !strings.Contains(resp4, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected python script creation, got: %s", resp4)
	}

	// 5. Agent Role - Step 4 (Result of creation)
	agentPrompt4 := agentPrompt3 + "\n\ncommand output:\ncat << 'EOF' > primes.py..."
	resp5, err := agent.Send(ctx, agentPrompt4)
	if err != nil {
		t.Fatalf("Agent prompt 4 failed: %v", err)
	}
	if !strings.Contains(resp5, "python3 primes.py") {
		t.Errorf("Expected python run, got: %s", resp5)
	}

	// 6. Agent Role - Step 5 (Result of run)
	agentPrompt5 := agentPrompt4 + "\n\ncommand output:\npython3 primes.py..."
	resp6, err := agent.Send(ctx, agentPrompt5)
	if err != nil {
		t.Fatalf("Agent prompt 5 failed: %v", err)
	}
	if !strings.Contains(resp6, "agent-bridge feature set") {
		t.Errorf("Expected agent-bridge calls, got: %s", resp6)
	}

	// 7. Agent Role - Step 6 (Result of bridge)
	agentPrompt6 := agentPrompt5 + "\n\ncommand output:\nagent-bridge feature set..."
	resp7, err := agent.Send(ctx, agentPrompt6)
	if err != nil {
		t.Fatalf("Agent prompt 6 failed: %v", err)
	}
	if !strings.Contains(resp7, "git commit") {
		t.Errorf("Expected git commit, got: %s", resp7)
	}

	// 8. Agent Role - Step 7 (Result of commit)
	agentPrompt7 := agentPrompt6 + "\n\ncommand output:\ngit commit..."
	resp8, err := agent.Send(ctx, agentPrompt7)
	if err != nil {
		t.Fatalf("Agent prompt 7 failed: %v", err)
	}
	if !strings.Contains(resp8, "task complete") {
		t.Errorf("Expected completion message, got: %s", resp8)
	}
}
