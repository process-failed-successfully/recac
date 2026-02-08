package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Default(t *testing.T) {
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

func TestMockAgent_TPM(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are the Technical Program Manager. Break down the requirements."
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "PRIMES") {
		t.Error("Expected JSON with project name")
	}
	if !strings.Contains(response, "req-the-makefile-targets-are-implemented") {
		t.Error("Expected specific feature ID")
	}
}

func TestMockAgent_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "## YOUR ROLE - CODING AGENT\nTask: Implement primes.py"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Error("Expected file creation script")
	}
	if !strings.Contains(response, "agent-bridge feature set") {
		t.Error("Expected agent-bridge call")
	}
}

func TestMockAgent_SmokeTestLogic(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer
	resp, _ := agent.Send(ctx, "## YOUR ROLE - INITIALIZER AGENT")
	if !strings.Contains(resp, "agent-bridge import") {
		t.Error("Initializer should return import script")
	}
	if !strings.Contains(resp, "req-primes") {
		t.Error("Initializer should include req-primes")
	}

	// 2. Setup Repo
	resp, _ = agent.Send(ctx, "## YOUR ROLE - CODING AGENT\nTask: req-setup-repo")
	if !strings.Contains(resp, "git init") {
		t.Error("Setup Repo should return git init")
	}
	if !strings.Contains(resp, "req-setup-repo") {
		t.Error("Setup Repo should signal req-setup-repo")
	}

	// 3. Setup CI
	resp, _ = agent.Send(ctx, "## YOUR ROLE - CODING AGENT\nTask: req-ci-workflow")
	if !strings.Contains(resp, ".github/workflows") {
		t.Error("CI Workflow should create .github directory")
	}

	// 4. Implement Tests
	resp, _ = agent.Send(ctx, "## YOUR ROLE - CODING AGENT\nTask: req-implement-tests")
	if !strings.Contains(resp, "test_primes.py") {
		t.Error("Implement Tests should create test file")
	}

	// 5. Implement Primes (via ID or filename)
	resp, _ = agent.Send(ctx, "## YOUR ROLE - CODING AGENT\nTask: req-implement-primes")
	if !strings.Contains(resp, "primes.py") {
		t.Error("Implement Primes should create primes.py")
	}
	// Check that it writes json
	if !strings.Contains(resp, "json.dump") {
		t.Error("Implement Primes should write json output")
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
