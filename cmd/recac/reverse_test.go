package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
)

// MockAgentForReverse implements agent.Agent
type MockAgentForReverse struct {
	Response string
}

func (m *MockAgentForReverse) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForReverse) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestReverseCmd(t *testing.T) {
	// Setup Temp Codebase
	tmpDir, err := os.MkdirTemp("", "recac-reverse-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dummy files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# My App"), 0644)

	outputFile := filepath.Join(tmpDir, "app_spec.txt")
	expectedSpec := "# Spec\nThis is a generated spec."

	// Mock Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForReverse{Response: expectedSpec}, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Override exit to prevent test failure
	origExit := exit
	exit = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("Exit called with code %d", code))
		}
	}
	defer func() { exit = origExit }()

	// Run Command
	cmd := reverseCmd
	// Set flags manually since we are calling runReverseCmd directly
	cmd.Flags().Set("path", tmpDir)
	cmd.Flags().Set("output", outputFile)

	// Execute Run
	runReverseCmd(cmd, []string{})

	// Verify Output
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != expectedSpec {
		t.Errorf("Expected content %q, got %q", expectedSpec, string(content))
	}
}
