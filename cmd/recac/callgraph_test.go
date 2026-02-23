package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFilterGraph(t *testing.T) {
	nodes := map[string]*analysis.CallGraphNode{
		"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA"},
		"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB"},
		"pkg.FuncC": {ID: "pkg.FuncC", Name: "FuncC"},
		"pkg.FuncD": {ID: "pkg.FuncD", Name: "FuncD"},
	}
	edges := []analysis.CallGraphEdge{
		{From: "pkg.FuncA", To: "pkg.FuncB"},
		{From: "pkg.FuncB", To: "pkg.FuncC"},
		{From: "pkg.FuncC", To: "pkg.FuncD"},
	}
	cg := &analysis.CallGraph{Nodes: nodes, Edges: edges}

	tests := []struct {
		name        string
		focus       string
		expectedLen int
	}{
		{
			name:        "No filter (empty focus)",
			focus:       "",
			expectedLen: 4, // Matches everything
		},
		{
			name:        "Filter by FuncB",
			focus:       "FuncB",
			expectedLen: 3, // FuncB + Caller(A) + Callee(C)
		},
		{
			name:        "Filter by FuncA",
			focus:       "FuncA",
			expectedLen: 2, // FuncA + Callee(B)
		},
		{
			name:        "Filter by FuncD",
			focus:       "FuncD",
			expectedLen: 2, // FuncD + Caller(C)
		},
		{
			name:        "No match",
			focus:       "NonExistent",
			expectedLen: 4, // Returns original graph
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterGraph(cg, tt.focus)
			if len(filtered.Nodes) != tt.expectedLen {
				t.Errorf("expected %d nodes, got %d", tt.expectedLen, len(filtered.Nodes))
			}
		})
	}
}

func TestGenerateMermaidCallGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA"},
			"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.FuncA", To: "pkg.FuncB"},
		},
	}

	output := generateMermaidCallGraph(cg)

	if !strings.Contains(output, "graph LR") {
		t.Error("expected 'graph LR'")
	}
	// Sanitize ID logic: likely replaces dots with underscores or similar
	// Let's check for the presence of the names at least
	if !strings.Contains(output, "FuncA") {
		t.Error("expected FuncA label")
	}
	if !strings.Contains(output, "FuncB") {
		t.Error("expected FuncB label")
	}
}

func TestRunCallGraph(t *testing.T) {
	// Create a temp dir with some Go code
	tmpDir := t.TempDir()

	mainGo := `package main

func main() {
	Helper()
}

func Helper() {
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Save global vars
	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()

	callGraphDir = tmpDir
	callGraphFocus = ""

	// Create command
	cmd := &cobra.Command{Use: "callgraph", RunE: runCallGraph}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := runCallGraph(cmd, []string{}); err != nil {
		t.Fatalf("runCallGraph failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "graph LR") {
		t.Error("expected mermaid graph output")
	}
	if !strings.Contains(output, "main_main") && !strings.Contains(output, "main.main") {
		t.Error("expected main.main (or sanitized) in output")
	}
}
