package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_PrimesHeuristic(t *testing.T) {
	m := NewMockAgent()

	prompt := "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000"
	resp, err := m.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "primes.py") {
		t.Errorf("Response should contain 'primes.py'")
	}
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Errorf("Response should contain bash block to create file")
	}
	if !strings.Contains(resp, "import json") {
		t.Errorf("Response should contain python import json")
	}
	if !strings.Contains(resp, "get_primes(10000)") {
		t.Errorf("Response should call get_primes with 10000")
	}
}

func TestMockAgent_FeatureListHeuristic(t *testing.T) {
	m := NewMockAgent()

	prompt := "You are a Technical Program Manager. Please generate feature_list.json"
	resp, err := m.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "feature_list.json") {
		t.Errorf("Response should contain 'feature_list.json'")
	}
}

func TestMockAgent_Default(t *testing.T) {
	m := NewMockAgent()

	prompt := "Hello"
	resp, err := m.Send(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(resp, "I received your prompt") {
		t.Errorf("Response should be generic")
	}
}
