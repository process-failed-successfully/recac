package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Basics(t *testing.T) {
	agent := NewMockAgent()

	prompt := "Generic prompt without special keywords"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

    // Check for no-op to avoid loop
    if !strings.Contains(response, "# no-op") {
        t.Errorf("Response missing no-op comment, got: %s", response)
    }
}

func TestMockAgent_Scenarios(t *testing.T) {
    agent := NewMockAgent()
    ctx := context.Background()

    // 1. Initializer Role -> Should return JSON tickets
    t.Run("Initializer", func(t *testing.T) {
        prompt := "## YOUR ROLE - INITIALIZER AGENT\nAnalyze the spec..."
        resp, _ := agent.Send(ctx, prompt)
        if !strings.Contains(resp, `"title": "Implement Prime Number Generator"`) {
            t.Errorf("Expected JSON ticket, got: %s", resp)
        }
    })

    // 2. Developer Role -> Should return Bash Script
    t.Run("Developer Implementation", func(t *testing.T) {
        prompt := "Create a Python script that calculates prime numbers"
        resp, _ := agent.Send(ctx, prompt)
        if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
            t.Errorf("Expected bash script, got: %s", resp)
        }
    })

    // 3. Developer Role (Repeated / Code Exists) -> Should return TRIGGER_QA
    t.Run("Developer Finished", func(t *testing.T) {
        prompt := "Create a Python script that calculates prime numbers\n\nExisting code:\ndef is_prime(n):"
        resp, _ := agent.Send(ctx, prompt)
        if resp != "TRIGGER_QA" {
            t.Errorf("Expected TRIGGER_QA, got: %s", resp)
        }
    })

    // 4. QA Role -> Should return QA_PASSED
    t.Run("QA Agent", func(t *testing.T) {
        prompt := "## YOUR ROLE - QA AGENT\nVerify the code..."
        resp, _ := agent.Send(ctx, prompt)
        if resp != "QA_PASSED" {
            t.Errorf("Expected QA_PASSED, got: %s", resp)
        }
    })

    // 5. Manager Role -> Should return PROJECT_SIGNED_OFF
    t.Run("Manager Agent", func(t *testing.T) {
        prompt := "## YOUR ROLE - PROJECT MANAGER\nReview progress..."
        resp, _ := agent.Send(ctx, prompt)
        if resp != "PROJECT_SIGNED_OFF" {
            t.Errorf("Expected PROJECT_SIGNED_OFF, got: %s", resp)
        }
    })
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
