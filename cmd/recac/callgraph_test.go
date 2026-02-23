package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"recac/internal/analysis"
)

func TestRunCallGraph(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-callgraph-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a sample Go file
	mainGo := `package main

func main() {
	foo()
}

func foo() {
	bar()
}

func bar() {
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Mock Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Set global variable callGraphDir (as runCallGraph uses it)
	// We need to restore it after test
	oldDir := callGraphDir
	defer func() { callGraphDir = oldDir }()
	callGraphDir = tmpDir

	// Set focus to empty
	oldFocus := callGraphFocus
	defer func() { callGraphFocus = oldFocus }()
	callGraphFocus = ""

	if err := runCallGraph(cmd, []string{}); err != nil {
		t.Fatalf("runCallGraph failed: %v", err)
	}

	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, "graph LR") {
		t.Error("Output should contain 'graph LR'")
	}

	// sanitizeMermaidID replaces '.' with '_'
	// "main.main" -> "main_main"
	// "main.foo" -> "main_foo"

	// Check for approximate edges
	if !strings.Contains(output, "main_main --> main_foo") {
		t.Error("Output should contain edge main_main --> main_foo")
	}
	if !strings.Contains(output, "main_foo --> main_bar") {
		t.Error("Output should contain edge main_foo --> main_bar")
	}

	// Just check if function names are present in labels
	// Labels include package name for local calls in root: "main.main"
	if !strings.Contains(output, "[\"main.main\"]") {
		t.Error("Output should contain main.main label")
	}
	if !strings.Contains(output, "[\"main.foo\"]") {
		t.Error("Output should contain main.foo label")
	}
	if !strings.Contains(output, "[\"main.bar\"]") {
		t.Error("Output should contain main.bar label")
	}
}

func TestFilterGraph(t *testing.T) {
	// Create a sample graph
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"A": {ID: "A", Name: "FuncA"},
			"B": {ID: "B", Name: "FuncB"},
			"C": {ID: "C", Name: "FuncC"},
			"D": {ID: "D", Name: "FuncD"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
			{From: "C", To: "D"},
		},
	}

	// Filter for "FuncB"
	filtered := filterGraph(cg, "FuncB")

	// Expect A->B and B->C (1 level depth)
	if len(filtered.Nodes) != 3 {
		t.Errorf("Expected 3 nodes (A, B, C), got %d", len(filtered.Nodes))
	}
	if _, ok := filtered.Nodes["A"]; !ok {
		t.Error("Node A missing")
	}
	if _, ok := filtered.Nodes["B"]; !ok {
		t.Error("Node B missing")
	}
	if _, ok := filtered.Nodes["C"]; !ok {
		t.Error("Node C missing")
	}
	if _, ok := filtered.Nodes["D"]; ok {
		t.Error("Node D should be excluded")
	}
}
