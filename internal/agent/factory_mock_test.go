package agent

import (
	"testing"
)

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create mock agent: %v", err)
	}

	if agent == nil {
		t.Fatal("Mock agent is nil")
	}

	// Verify type
	if _, ok := agent.(*MockAgent); !ok {
		t.Errorf("Expected *MockAgent, got %T", agent)
	}
}
