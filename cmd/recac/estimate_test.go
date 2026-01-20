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

// MockEstimateAgent implements agent.Agent interface
type MockEstimateAgent struct {
	Response string
	Err      error
}

func (m *MockEstimateAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockEstimateAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestEstimateCommand(t *testing.T) {
	// Setup global test env logic (similar to TestCommands in commands_test.go)
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir) // Change to temp dir to avoid file system interference
	defer os.Chdir(originalWd)

	// Setup mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("Basic Estimation", func(t *testing.T) {
		mockResponse := `{
			"summary": "Refactor logic",
			"complexity": "Medium",
			"story_points": 5,
			"estimated_hours": "4-8h",
			"risks": ["Breaking changes"],
			"implementation_steps": ["Step 1", "Step 2"]
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockEstimateAgent{Response: mockResponse}, nil
		}

		output, err := executeCommand(rootCmd, "estimate", "My Task")
		if err != nil {
			t.Errorf("Estimate failed: %v", err)
		}

		if !strings.Contains(output, "Medium") {
			t.Errorf("Expected output to contain 'Medium', got %s", output)
		}
		if !strings.Contains(output, "5") {
			t.Errorf("Expected output to contain '5', got %s", output)
		}
		if !strings.Contains(output, "Breaking changes") {
			t.Errorf("Expected output to contain 'Breaking changes', got %s", output)
		}
	})

	t.Run("JSON Output", func(t *testing.T) {
		mockResponse := `{
			"summary": "Simple fix",
			"complexity": "Low",
			"story_points": 1,
			"estimated_hours": "1h",
			"risks": [],
			"implementation_steps": []
		}`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockEstimateAgent{Response: mockResponse}, nil
		}

		output, err := executeCommand(rootCmd, "estimate", "Small task", "--json")
		if err != nil {
			t.Errorf("Estimate JSON failed: %v", err)
		}

		if !strings.Contains(output, `"complexity": "Low"`) {
			t.Errorf("Expected JSON output, got %s", output)
		}
	})

	t.Run("With Context", func(t *testing.T) {
		// Create a dummy file to focus on
		focusFile := filepath.Join(tmpDir, "dummy.go")
		if err := os.WriteFile(focusFile, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}

		mockResponse := `{"summary": "Contextual", "complexity": "Low", "story_points": 1, "estimated_hours": "1h", "risks": [], "implementation_steps": []}`
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockEstimateAgent{Response: mockResponse}, nil
		}

		// We use relative path "." since we Chdir'd to tmpDir
		output, err := executeCommand(rootCmd, "estimate", "Task", "--focus", ".")
		if err != nil {
			t.Errorf("Estimate with focus failed: %v", err)
		}
		if !strings.Contains(output, "Contextual") {
			t.Errorf("Expected output to contain 'Contextual', got %s", output)
		}
	})

	t.Run("Agent Error", func(t *testing.T) {
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockEstimateAgent{Err: fmt.Errorf("agent down")}, nil
		}

		_, err := executeCommand(rootCmd, "estimate", "Fail")
		if err == nil {
			t.Error("Expected error when agent fails")
		}
	})

	t.Run("Bad JSON Response", func(t *testing.T) {
		// If agent returns garbage, we should handle it (print raw output)
		mockResponse := `Not valid JSON`
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockEstimateAgent{Response: mockResponse}, nil
		}

		output, err := executeCommand(rootCmd, "estimate", "Garbage")
		if err != nil {
			t.Errorf("Estimate should not fail on bad JSON, just warn: %v", err)
		}
		if !strings.Contains(output, "Raw Response") {
			t.Error("Expected output to contain 'Raw Response' fallback")
		}
	})
}
