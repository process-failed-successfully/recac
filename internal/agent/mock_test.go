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

    if !strings.Contains(response, "```bash\n# no-op\n```") {
        t.Errorf("Response missing no-op block, got: %s", response)
    }
}

func TestMockAgent_Tickets(t *testing.T) {
    agent := NewMockAgent()
    prompt := "Please generate tickets Type: Task"

    response, err := agent.Send(context.Background(), prompt)
    if err != nil {
        t.Fatalf("Send failed: %v", err)
    }

    if !strings.Contains(response, "ID:[PRIMES]") {
        t.Errorf("Response missing ID:[PRIMES], got: %s", response)
    }
    if !strings.Contains(response, "Calculate Primes") {
        t.Errorf("Response missing title, got: %s", response)
    }
}

func TestMockAgent_Initialization(t *testing.T) {
    agent := NewMockAgent()
    prompt := "Please initialize the project with Feature List"

    response, err := agent.Send(context.Background(), prompt)
    if err != nil {
        t.Fatalf("Send failed: %v", err)
    }

    if !strings.Contains(response, "agent-bridge import feature_list.json") {
        t.Errorf("Response missing agent-bridge import, got: %s", response)
    }
    if !strings.Contains(response, "cat <<EOF > feature_list.json") {
        t.Errorf("Response missing feature_list.json creation, got: %s", response)
    }
}

func TestMockAgent_Primes(t *testing.T) {
    agent := NewMockAgent()
    prompts := []string{
        "Please implement primes.py",
        "req-primes task",
        "work on [PRIMES]",
        "work on ID:[PRIMES]",
    }

    for _, prompt := range prompts {
        response, err := agent.Send(context.Background(), prompt)
        if err != nil {
            t.Fatalf("Send failed for prompt '%s': %v", prompt, err)
        }

        if !strings.Contains(response, "cat <<EOF > primes.py") {
            t.Errorf("Response missing primes.py creation for prompt '%s'", prompt)
        }
        if !strings.Contains(response, "python3 primes.py") {
            t.Errorf("Response missing python execution for prompt '%s'", prompt)
        }
        if !strings.Contains(response, "git commit") {
            t.Errorf("Response missing git commit for prompt '%s'", prompt)
        }
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
