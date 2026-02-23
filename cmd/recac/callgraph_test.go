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
	// 1. Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "recac-callgraph-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create files
	mainGo := `package main

func main() {
	Helper()
}

func Helper() {
	println("Hello")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Setup command
	cmd := &cobra.Command{Use: "callgraph"}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// 4. Run command
	// We need to set the global variable callGraphDir
	originalDir := callGraphDir
	callGraphDir = tmpDir
	defer func() { callGraphDir = originalDir }()

	if err := runCallGraph(cmd, []string{}); err != nil {
		t.Fatalf("runCallGraph failed: %v", err)
	}

	// 5. Verify output
	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, "graph LR") {
		t.Error("Output should contain 'graph LR'")
	}
	if !strings.Contains(output, "main.main") {
		t.Error("Output should contain 'main.main'")
	}
	if !strings.Contains(output, "main.Helper") {
		t.Error("Output should contain 'main.Helper'")
	}
	// Check edge
	// Mermaid syntax: A --> B
	// Since IDs are sanitized, we need to be careful.
	// But simple names should be fine.
	// Actually, sanitizeMermaidID replaces '.' with '_'.
	// So "main.main" becomes "main_main".
	// Let's check for "main_main"
	if !strings.Contains(output, "main_main") {
		t.Error("Output should contain sanitized ID 'main_main'")
	}
}

func TestRunCallGraph_Focus(t *testing.T) {
	// 1. Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "recac-callgraph-focus-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create files
	mainGo := `package main

func main() {
	FocusTarget()
	Other()
}

func FocusTarget() {
	println("Focus")
}

func Other() {
	println("Other")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Setup command
	cmd := &cobra.Command{Use: "callgraph"}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// 4. Run command with focus
	originalDir := callGraphDir
	callGraphDir = tmpDir
	originalFocus := callGraphFocus
	callGraphFocus = "FocusTarget"
	defer func() {
		callGraphDir = originalDir
		callGraphFocus = originalFocus
	}()

	if err := runCallGraph(cmd, []string{}); err != nil {
		t.Fatalf("runCallGraph failed: %v", err)
	}

	// 5. Verify output
	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, "FocusTarget") {
		t.Error("Output should contain focused function")
	}
	// Other should NOT be in the graph if it's not connected to FocusTarget?
	// Wait, Other is called by main, and main calls FocusTarget.
	// If we filter by FocusTarget, we include callers and callees.
	// main calls FocusTarget, so main is included.
	// main also calls Other. Does filterGraph include siblings?
	// The implementation says:
	// "Find edges connected to relevant nodes" (FocusTarget)
	// FocusTarget <- main. So this edge is included.
	// Does it include main -> Other?
	// "Expanded nodes" includes main.
	// Then we iterate edges again? No.
	// We only iterate edges once: "for _, edge := range cg.Edges".
	// If edge is connected to relevant nodes (FocusTarget).
	// main->FocusTarget is connected to FocusTarget. So included.
	// main->Other is NOT connected to FocusTarget directly.
	// So it should NOT be included unless logic expands to 2 levels or full graph of expanded nodes.
	// The code:
	// if relevantNodes[edge.From] || relevantNodes[edge.To]
	// relevantNodes only has FocusTarget initially.
	// So only edges touching FocusTarget.
	// So main->Other is NOT included.

	if strings.Contains(output, "Other") {
		// Wait, "Other" might be in the label of "main.Other" node if it was included.
		// But verify logic:
		// filteredEdges appends if relevantNodes[From] or relevantNodes[To].
		// relevantNodes only contains "FocusTarget".
		// main -> FocusTarget: To is relevant. Included.
		// main -> Other: From is main (not relevant yet), To is Other (not relevant).
		// So not included.
		// So "Other" function should not appear.
		t.Error("Output should NOT contain unrelated function 'Other'")
	}
}
