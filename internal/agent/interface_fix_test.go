package agent

import (
	"testing"
)

func TestNewAgent_Mock(t *testing.T) {
	agent, err := NewAgent("mock", "", "", "", "")
	if err != nil {
		t.Fatalf("NewAgent('mock') failed: %v", err)
	}
	if agent == nil {
		t.Fatal("NewAgent('mock') returned nil agent")
	}

	_, ok := agent.(*MockAgent)
	if !ok {
		t.Errorf("NewAgent('mock') returned %T, expected *MockAgent", agent)
	}
}
