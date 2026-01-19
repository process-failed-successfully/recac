package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTodoScanCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-scan-test")
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

	// Create some files with TODOs
	files := map[string]string{
		"main.go": `package main
// TODO: Implement main logic
func main() {
	// FIXME: Handle error
}`,
		"script.py": `# TODO: Remove hardcoded paths
import os`,
		"README.md": `# Project
<!-- TODO: Add usage instructions -->
`,
		"TODO.md":    "# TODO\n", // Should be ignored
	}

	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create ignored directories
	if err := os.Mkdir("node_modules", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("node_modules/ignored.js", []byte("// TODO: This should be ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(".hidden", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".hidden/secret.go", []byte("// TODO: Secret todo"), 0644); err != nil {
		t.Fatal(err)
	}

	// Helper to capture output
	execute := func(cmdArgs []string) (string, error) {
		// Reset flags if any (none for scan currently)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs(cmdArgs)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// Run scan
	t.Run("Scan TODOs", func(t *testing.T) {
		out, err := execute([]string{"todo", "scan"})
		if err != nil {
			t.Fatalf("Failed to scan: %v", err)
		}

		if !strings.Contains(out, "Added") {
			t.Errorf("Expected output to mention added tasks, got: %s", out)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}

		sContent := string(content)
		// Check for expected tasks
		expected := []string{
			"Implement main logic",
			"Handle error",
			"Remove hardcoded paths",
			"Add usage instructions",
		}

		for _, exp := range expected {
			if !strings.Contains(sContent, exp) {
				t.Errorf("Expected task '%s' not found in TODO.md", exp)
			}
		}

		// Check for ignored tasks
		if strings.Contains(sContent, "Secret todo") {
			t.Errorf("Found task from hidden directory")
		}
		if strings.Contains(sContent, "This should be ignored") {
			t.Errorf("Found task from node_modules")
		}
	})

	// Run scan again (idempotency)
	t.Run("Scan Idempotency", func(t *testing.T) {
		out, err := execute([]string{"todo", "scan"})
		if err != nil {
			t.Fatalf("Failed to scan: %v", err)
		}

		if !strings.Contains(out, "No new tasks added") {
			content, _ := os.ReadFile("TODO.md")
			t.Errorf("Expected no new tasks, got: %s. TODO.md content:\n%s", out, string(content))
		}
	})

	// Test adding a new file and scanning again
	t.Run("Scan New File", func(t *testing.T) {
		if err := os.WriteFile("new.go", []byte("// BUG: fix leak"), 0644); err != nil {
			t.Fatal(err)
		}

		out, err := execute([]string{"todo", "scan"})
		if err != nil {
			t.Fatalf("Failed to scan: %v", err)
		}

		if !strings.Contains(out, "Added 1 new tasks") {
			t.Errorf("Expected 1 new task, got: %s", out)
		}

		content, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "fix leak") {
			t.Errorf("New task not found")
		}
	})
}
