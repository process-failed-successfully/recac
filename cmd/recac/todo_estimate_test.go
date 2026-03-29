package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"
)

// MockTodoEstimateAgent implements agent.Agent interface for this test
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

func TestTodoEstimateCommand(t *testing.T) {
	// Setup global test env logic
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir) // Change to temp dir
	defer os.Chdir(originalWd)

	// Setup mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	// Create dummy target file
	targetFile := "dummy.go"
	targetContent := "package main\n\nfunc main() {\n  // TODO: Fix me\n}\n"
	if err := os.WriteFile(targetFile, []byte(targetContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create TODO.md with a task pointing to dummy.go
	// Format: - [ ] [dummy.go:3] Fix the thing
	todoContent := fmt.Sprintf("- [ ] [%s:3] Fix the thing\n", targetFile)
	if err := os.WriteFile("TODO.md", []byte(todoContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Estimate Existing TODO", func(t *testing.T) {
		mockResponse := `{
			"summary": "Fix logic",
			"complexity": "Low",
			"story_points": 2,
			"estimated_hours": "1h",
			"risks": [],
			"implementation_steps": ["Step 1"]
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		// We pass "1" as the index (first item)
		output, err := executeCommand(rootCmd, "todo", "estimate", "1")
		if err != nil {
			t.Errorf("Todo estimate failed: %v", err)
		}

		// Check if output contains key info
		if !strings.Contains(output, "Fix logic") {
			t.Errorf("Expected output to contain 'Fix logic', got %s", output)
		}
		if !strings.Contains(output, "Low") {
			t.Errorf("Expected output to contain 'Low', got %s", output)
		}
		if !strings.Contains(output, "1h") {
			t.Errorf("Expected output to contain '1h', got %s", output)
		}
		// Check context was found
		if !strings.Contains(output, "Context: dummy.go:3") {
			t.Errorf("Expected output to show context, got %s", output)
		}
	})

	t.Run("Estimate With JSON", func(t *testing.T) {
		mockResponse := `{
			"summary": "JSON Test",
			"complexity": "High",
			"story_points": 8,
			"estimated_hours": "5h",
			"risks": [],
			"implementation_steps": []
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		output, err := executeCommand(rootCmd, "todo", "estimate", "1", "--json")
		if err != nil {
			t.Errorf("Todo estimate JSON failed: %v", err)
		}

		if !strings.Contains(output, `"complexity": "High"`) {
			t.Errorf("Expected JSON output, got %s", output)
		}
	})

	t.Run("Invalid Index", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "todo", "estimate", "99")
		if err == nil {
			t.Error("Expected error for invalid index")
		}
	})
}
