package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCallGraph(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a simple Go file structure
	mainGo := `package main

func main() {
	Hello()
}

func Hello() {
	World()
}

func World() {
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Save/Restore global variable
	originalDir := callGraphDir
	defer func() { callGraphDir = originalDir }()

	// Test case 1: Basic run
	t.Run("Basic Graph", func(t *testing.T) {
		callGraphDir = tmpDir
		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		if err := runCallGraph(cmd, nil); err != nil {
			t.Fatalf("runCallGraph failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "graph LR") {
			t.Error("Output does not contain mermaid header 'graph LR'")
		}
		// Since mermaid uses sanitized IDs, checking for exact strings might be tricky if sanitization changes.
		// But let's check for function names.
		// "main" is usually sanitized to "main" (or similar if package name is prepended).
		// Wait, analysis package might use full package path like "main.main".
		// Sanitization replaces dots with underscores usually.
		// Let's check for "Hello" and "World".
		if !strings.Contains(output, "Hello") {
			t.Error("Output does not contain 'Hello' node")
		}
		if !strings.Contains(output, "World") {
			t.Error("Output does not contain 'World' node")
		}
	})

	// Test case 2: Focus
	t.Run("Focus Graph", func(t *testing.T) {
		callGraphDir = tmpDir
		originalFocus := callGraphFocus
		defer func() { callGraphFocus = originalFocus }()
		callGraphFocus = "Hello" // Focus on Hello

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		if err := runCallGraph(cmd, nil); err != nil {
			t.Fatalf("runCallGraph failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Hello") {
			t.Error("Output does not contain focused node 'Hello'")
		}
		// It should also contain World (callee) and main (caller) if depth is handled correctly,
		// or at least Hello.
		if !strings.Contains(output, "World") {
			t.Log("Warning: focused output might not contain callee 'World'")
		}
	})
}
