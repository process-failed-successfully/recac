package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptCommand(t *testing.T) {
	// 1. Test List
	t.Run("list", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "prompt", "list")
		if err != nil {
			t.Fatalf("executeCommand(prompt list) failed: %v", err)
		}
		if !strings.Contains(output, "coding_agent") {
			t.Errorf("Expected output to contain 'coding_agent', got: %s", output)
		}
	})

	// 2. Test Show
	t.Run("show", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "prompt", "show", "coding_agent")
		if err != nil {
			t.Fatalf("executeCommand(prompt show) failed: %v", err)
		}
		// The prompt should contain some expected content like "role" or "Task"
		// coding_agent usually contains "coding agent" or similar.
		if !strings.Contains(output, "{") {
			// Basic check for placeholders
			t.Errorf("Expected output to contain placeholders, got: %s", output)
		}
	})

	// 3. Test Override
	t.Run("override", func(t *testing.T) {
		tmpDir := t.TempDir()
		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		os.Chdir(tmpDir)

		output, err := executeCommand(rootCmd, "prompt", "override", "coding_agent")
		if err != nil {
			t.Fatalf("executeCommand(prompt override) failed: %v", err)
		}

		expectedFile := "coding_agent.md"
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be created", expectedFile)
		}

		content, _ := os.ReadFile(expectedFile)
		if len(content) == 0 {
			t.Error("Created file is empty")
		}

		if !strings.Contains(output, "saved to") {
			t.Errorf("Expected output to confirm save, got: %s", output)
		}

		// Test override with --out
		customOut := filepath.Join(tmpDir, "custom_prompts")
		os.Mkdir(customOut, 0755)

		output, err = executeCommand(rootCmd, "prompt", "override", "planner", "--out", customOut)
		if err != nil {
			t.Fatalf("executeCommand(prompt override --out) failed: %v", err)
		}

		expectedCustomFile := filepath.Join(customOut, "planner.md")
		if _, err := os.Stat(expectedCustomFile); os.IsNotExist(err) {
			t.Errorf("Expected file %s to be created", expectedCustomFile)
		}
	})
}
