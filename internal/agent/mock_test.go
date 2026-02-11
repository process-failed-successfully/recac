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

func TestMockAgent_Initializer(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - INITIALIZER AGENT\nRepo: https://github.com/test/repo"
	response, err := agent.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// Normal initializer (no primes) checks for git setup
	if !strings.Contains(response, "git init") {
		t.Error("Expected 'git init' in response")
	}
	if !strings.Contains(response, "git remote add origin") {
		t.Error("Expected 'git remote add origin' in response")
	}
}

func TestMockAgent_PrimesScenario(t *testing.T) {
	agent := NewMockAgent()

	// 1. Initializer with ID (Must include "prime" to trigger block)
	prompt1 := "ROLE - INITIALIZER AGENT\nSpec:\nID: [MFLP-123]\nDescription: Implement primes."
	resp1, err := agent.Send(context.Background(), prompt1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp1, `"id": "MFLP-123"`) {
		t.Errorf("Expected Feature ID MFLP-123 in Initializer response, got:\n%s", resp1)
	}

	// 2. Coding Agent with ID (Must include "primes.py" to trigger block)
	// Test various formats
	prompts := []string{
		"YOUR ROLE - CODING AGENT\nYour assigned task is **MFLP-123**\nFeature ID: [MFLP-123]\nImplement primes.py",
		"YOUR ROLE - CODING AGENT\nFeature ID: MFLP-123\nImplement primes.py",
		"YOUR ROLE - CODING AGENT\n**Feature ID**: MFLP-123\nImplement primes.py",
	}

	for _, p := range prompts {
		resp, err := agent.Send(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(resp, "git checkout -B agent/MFLP-123") {
			t.Errorf("Expected branch agent/MFLP-123 for prompt:\n%s\nGot:\n%s", p, resp)
		}
		if !strings.Contains(resp, "agent-bridge feature set MFLP-123") {
			t.Errorf("Expected feature set MFLP-123 for prompt:\n%s\nGot:\n%s", p, resp)
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
