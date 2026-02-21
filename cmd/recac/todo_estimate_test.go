package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
)

func TestCleanTaskDescription(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"- [ ] Simple task", "Simple task"},
		{"- [x] Done task", "Done task"},
		{"- [ ] [main.go:10] TODO: Refactor", "TODO: Refactor"},
		{"- [ ] [pkg/api.go:23] FIXME: Bug here", "FIXME: Bug here"},
		{"- [ ]   [file:1]   Task with spaces", "Task with spaces"},
	}

	for _, tt := range tests {
		got := cleanTaskDescription(tt.input)
		if got != tt.expected {
			t.Errorf("cleanTaskDescription(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRemoveExistingEstimate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Task 1 **(Est: 3pts, Low)**", "Task 1"},
		{"Task 2 **(Est: 5pts, Medium)**", "Task 2"},
		{"Task 3", "Task 3"},
	}

	for _, tt := range tests {
		got := removeExistingEstimate(tt.input)
		if got != tt.expected {
			t.Errorf("removeExistingEstimate(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

// MockAgent for testing
type MockTodoEstimateAgent struct{}

func (m *MockTodoEstimateAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Return a valid JSON response
	return `{
		"summary": "Mock estimation",
		"complexity": "Medium",
		"story_points": 3,
		"estimated_hours": "2-4h",
		"risks": [],
		"implementation_steps": []
	}`, nil
}

func (m *MockTodoEstimateAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Not used in this command, but needed for interface compliance
	return m.Send(ctx, prompt)
}

func TestTodoEstimateCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-estimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current working directory and restore it after test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Mock agent factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockTodoEstimateAgent{}, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Helper to execute command
	execute := func(cmdArgs []string) (string, error) {
		// Reset flags
		todoEstimateAll = false
		todoEstimateForce = false

		rootCmd.SetArgs(cmdArgs)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// 1. Setup TODO.md
	if err := os.WriteFile("TODO.md", []byte("# TODO\n\n- [ ] Task 1\n- [ ] [main.go:10] Task 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("main.go", []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Estimate specific task
	t.Run("Estimate Single Task", func(t *testing.T) {
		out, err := execute([]string{"todo", "estimate", "1"})
		if err != nil {
			t.Fatalf("Failed to estimate task: %v\nOutput: %s", err, out)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "- [ ] Task 1 **(Est: 3pts, Medium)**") {
			t.Errorf("Task 1 not updated correctly: %s", string(content))
		}
	})

	// 3. Estimate All
	t.Run("Estimate All", func(t *testing.T) {
		// Task 1 is already estimated, Task 2 is not
		out, err := execute([]string{"todo", "estimate", "--all"})
		if err != nil {
			t.Fatalf("Failed to estimate all: %v\nOutput: %s", err, out)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}

		// Task 1 should remain (or be skipped)
		// Task 2 should be estimated
		if !strings.Contains(string(content), "- [ ] [main.go:10] Task 2 **(Est: 3pts, Medium)**") {
			t.Errorf("Task 2 not updated correctly: %s", string(content))
		}
	})

	// 4. Force Estimate
	t.Run("Force Estimate", func(t *testing.T) {
		// Both tasks are estimated now. Force re-estimation of Task 1.
		// We'll modify the mock to return something else? No, mock is static.
		// But we can check output logs "Analyzing Task 1..."

		out, err := execute([]string{"todo", "estimate", "--all", "--force"})
		if err != nil {
			t.Fatalf("Failed to force estimate: %v\nOutput: %s", err, out)
		}

		if !strings.Contains(out, "Analyzing Task 1") {
			t.Errorf("Should have re-analyzed Task 1 with --force. Output: %s", out)
		}
	})
}
