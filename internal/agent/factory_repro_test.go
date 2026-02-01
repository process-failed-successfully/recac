package agent

import (
	"testing"
)

func TestRepro(t *testing.T) {
	agent, err := NewAgent("mock", "dummy-key", "dummy-model", ".", "test-project")
	if err != nil {
		t.Fatalf("NewAgent failed for mock provider: %v", err)
	}

	if _, ok := agent.(*MockAgent); !ok {
		t.Errorf("Expected *MockAgent, got %T", agent)
	}
}
