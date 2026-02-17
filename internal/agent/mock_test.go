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

func TestMockAgent_Primes_Planning(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Technical Program Manager. Please plan the following: ID:[PRIMES] Prime Number Script. Repo: https://github.com/test/repo"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return JSON
	if !strings.Contains(response, `ID:[PRIMES] Create Prime Number Script`) {
		t.Errorf("Response missing JSON title, got: %s", response)
	}
	if !strings.Contains(response, `"type": "Task"`) {
		t.Errorf("Response missing Task type, got: %s", response)
	}
	if !strings.Contains(response, "https://github.com/test/repo") {
		t.Errorf("Response missing repo URL, got: %s", response)
	}
}

func TestMockAgent_Primes_Coding(t *testing.T) {
	agent := NewMockAgent()
	prompt := "You are an expert Software Engineer. Please solve the following: ID:[PRIMES] Prime Number Script."
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should return Python script
	if !strings.Contains(response, "primes.py") {
		t.Errorf("Response missing python script, got: %s", response)
	}
	if !strings.Contains(response, "cat << 'EOF' > primes.py") {
		t.Errorf("Response missing bash block, got: %s", response)
	}
	if strings.Contains(response, `"type": "Task"`) {
		t.Errorf("Response should NOT contain JSON task definition, got: %s", response)
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
