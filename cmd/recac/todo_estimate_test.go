package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"
)

// MockTodoEstimateAgent implements agent.Agent interface for this test file
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

func TestTodoEstimateCmd(t *testing.T) {
	// Setup global test env logic
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir) // Change to temp dir
	defer os.Chdir(originalWd)

	// Setup mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("Estimate Task With File Context", func(t *testing.T) {
		// 1. Create a dummy file
		dummyFile := "internal/logic.go"
		os.MkdirAll(filepath.Dir(dummyFile), 0755)
		content := "package logic\n\nfunc Process() {\n  // TODO: Implement this\n}"
		os.WriteFile(dummyFile, []byte(content), 0644)

		// 2. Create TODO.md
		todoContent := fmt.Sprintf("# TODO\n\n- [ ] [%s:4] Implement processing logic\n", dummyFile)
		os.WriteFile("TODO.md", []byte(todoContent), 0644)

		// 3. Mock Agent
		mockResponse := `{
			"summary": "Implement logic",
			"complexity": "Medium",
			"story_points": 3,
			"estimated_hours": "2h",
			"risks": [],
			"implementation_steps": []
		}`
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		// 4. Run Estimate
		output, err := executeCommand(rootCmd, "todo", "estimate", "1")
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// 5. Verify Output
		if !strings.Contains(output, "Analyzing TODO in internal/logic.go") {
			t.Errorf("Expected analysis message, got: %s", output)
		}
		if !strings.Contains(output, "3") { // Story points
			t.Errorf("Expected story points in output, got: %s", output)
		}

		// 6. Verify TODO.md update
		updatedContent, _ := os.ReadFile("TODO.md")
		if !strings.Contains(string(updatedContent), "(Est: 3pts, 2h)") {
			t.Errorf("TODO.md not updated correctly: %s", string(updatedContent))
		}
	})

	t.Run("Estimate Task Without Context", func(t *testing.T) {
		// 1. Create TODO.md with a task without file link
		todoContent := "# TODO\n\n- [ ] Generic task\n"
		os.WriteFile("TODO.md", []byte(todoContent), 0644)

		// 2. Mock Agent
		mockResponse := `{
			"summary": "Generic",
			"complexity": "Low",
			"story_points": 1,
			"estimated_hours": "1h",
			"risks": [],
			"implementation_steps": []
		}`
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		// 3. Run Estimate
		output, err := executeCommand(rootCmd, "todo", "estimate", "1")
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// 4. Verify Output
		if !strings.Contains(output, "No file context found") {
			t.Errorf("Expected no context message, got: %s", output)
		}

		// 5. Verify TODO.md update
		updatedContent, _ := os.ReadFile("TODO.md")
		if !strings.Contains(string(updatedContent), "(Est: 1pts, 1h)") {
			t.Errorf("TODO.md not updated correctly: %s", string(updatedContent))
		}
	})

	t.Run("Re-Estimate Updates Existing", func(t *testing.T) {
		// 1. Create TODO.md with existing estimate
		todoContent := "# TODO\n\n- [ ] Task (Est: 1pts, 1h)\n"
		os.WriteFile("TODO.md", []byte(todoContent), 0644)

		// 2. Mock Agent with DIFFERENT estimate
		mockResponse := `{
			"summary": "Harder",
			"complexity": "High",
			"story_points": 8,
			"estimated_hours": "10h",
			"risks": [],
			"implementation_steps": []
		}`
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTodoEstimateAgent{Response: mockResponse}, nil
		}

		// 3. Run Estimate
		_, err := executeCommand(rootCmd, "todo", "estimate", "1")
		if err != nil {
			t.Fatalf("Command failed: %v", err)
		}

		// 4. Verify TODO.md update
		updatedContent, _ := os.ReadFile("TODO.md")
		if !strings.Contains(string(updatedContent), "(Est: 8pts, 10h)") {
			t.Errorf("TODO.md not updated correctly: %s", string(updatedContent))
		}
		if strings.Contains(string(updatedContent), "(Est: 1pts, 1h)") {
			t.Errorf("Old estimate should be gone")
		}
	})
}
