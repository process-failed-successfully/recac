package main

import (
	"context"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"
)

// MockTodoEstimateAgent implements agent.Agent interface
type MockTodoEstimateAgent struct {
	Response string
	Err      error
}

func (m *MockTodoEstimateAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockTodoEstimateAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestTodoEstimate(t *testing.T) {
	// Setup
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	// Create TODO.md
	todoContent := []string{
		"# TODO",
		"",
		"- [ ] Task 1",
		"- [ ] [main.go:10] Task 2 with context",
		"- [x] Done task",
	}
	if err := os.WriteFile("TODO.md", []byte(strings.Join(todoContent, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	// Create context file
	if err := os.WriteFile("main.go", []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Estimate with Context", func(t *testing.T) {
		mockResponse := `{
			"summary": "Fix main",
			"complexity": "Low",
			"story_points": 1,
			"estimated_hours": "1h",
			"risks": [],
			"implementation_steps": []
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		// Index 2 corresponds to "Task 2 with context"
		output, err := executeCommand(rootCmd, "todo", "estimate", "2")
		if err != nil {
			t.Errorf("Todo estimate failed: %v", err)
		}

		if !strings.Contains(output, "Found context in main.go at line 10") {
			t.Errorf("Expected to find context, got output:\n%s", output)
		}
		if !strings.Contains(output, "Fix main") {
			t.Errorf("Expected output to contain summary, got:\n%s", output)
		}
		// Verify PrintEstimateReport output format
		if !strings.Contains(output, "Complexity:") {
			t.Errorf("Expected output to contain 'Complexity:', got:\n%s", output)
		}
	})

	t.Run("Estimate without Context", func(t *testing.T) {
		mockResponse := `{
			"summary": "Simple task",
			"complexity": "Low",
			"story_points": 1,
			"estimated_hours": "0.5h",
			"risks": [],
			"implementation_steps": []
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		output, err := executeCommand(rootCmd, "todo", "estimate", "1")
		if err != nil {
			t.Errorf("Todo estimate failed: %v", err)
		}

		if !strings.Contains(output, "No file context found") {
			t.Errorf("Expected no context found message, got:\n%s", output)
		}
	})

	t.Run("Invalid Index", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "todo", "estimate", "99")
		if err == nil {
			t.Error("Expected error for invalid index")
		}
	})
}
