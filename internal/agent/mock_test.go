package agent

import (
	"context"
	"encoding/json"
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

func TestMockAgent_Heuristics(t *testing.T) {
	agent := NewMockAgent()
	ctx := context.Background()

	// 1. Initializer (Legacy Prompt)
	resp, err := agent.Send(ctx, "You are the Initializer")
	if err != nil {
		t.Fatalf("Initializer (legacy) failed: %v", err)
	}
	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("Expected feature_list.json in Initializer (legacy) response, got: %s", resp)
	}

	// 1b. Initializer (New Prompt)
	resp, err = agent.Send(ctx, "## YOUR ROLE - INITIALIZER AGENT")
	if err != nil {
		t.Fatalf("Initializer (new) failed: %v", err)
	}
	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("Expected feature_list.json in Initializer (new) response, got: %s", resp)
	}

	// 2. TPM
	resp, err = agent.Send(ctx, "Technical Program Manager")
	if err != nil {
		t.Fatalf("TPM failed: %v", err)
	}
	if !strings.Contains(resp, "Implement Primes") {
		t.Errorf("Expected 'Implement Primes' in TPM response, got: %s", resp)
	}

	// Helper to strip markdown
	stripMarkdown := func(s string) string {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "```json") {
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimSuffix(s, "```")
		} else if strings.HasPrefix(s, "```") {
			s = strings.TrimPrefix(s, "```")
			s = strings.TrimSuffix(s, "```")
		}
		return strings.TrimSpace(s)
	}

	jsonStr := stripMarkdown(resp)

	// Verify strict JSON structure matches CLI expectation
	var tickets []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &tickets); err != nil {
		t.Fatalf("Failed to unmarshal TPM response: %v. Raw response: %s", err, resp)
	}
	if len(tickets) == 0 {
		t.Fatalf("Expected at least one ticket")
	}
	if _, ok := tickets[0]["title"]; !ok {
		t.Errorf("Expected 'title' field in ticket JSON, got: %v", tickets[0])
	}

	// 3. Project Manager
	resp, err = agent.Send(ctx, "PROJECT MANAGER")
	if err != nil {
		t.Fatalf("PM failed: %v", err)
	}
	if !strings.Contains(resp, "PROJECT_SIGNED_OFF") {
		t.Errorf("Expected PROJECT_SIGNED_OFF in PM response, got: %s", resp)
	}

	// 4. Coding Agent (Primes) - First call
	resp, err = agent.Send(ctx, "Implement [PRIMES] Prime Number Script")
	if err != nil {
		t.Fatalf("Coding Agent failed: %v", err)
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Expected bash script for primes.py in Coding Agent response, got: %s", resp)
	}
	if !strings.Contains(resp, "git commit") {
		t.Errorf("Expected git commit in Coding Agent response, got: %s", resp)
	}

	// 5. Coding Agent (Primes) - Second call (Success signal)
	resp, err = agent.Send(ctx, "Please continue [PRIMES]")
	if err != nil {
		t.Fatalf("Coding Agent (2nd call) failed: %v", err)
	}
	if !strings.Contains(resp, "QA_PASSED") {
		t.Errorf("Expected QA_PASSED in 2nd Coding Agent response, got: %s", resp)
	}

	// 6. QA Agent
	resp, err = agent.Send(ctx, "QA AGENT")
	if err != nil {
		t.Fatalf("QA Agent failed: %v", err)
	}
	if !strings.Contains(resp, "QA_PASSED") {
		t.Errorf("Expected QA_PASSED in QA Agent response, got: %s", resp)
	}
}
